# Wallet & Transaksi Saldo API

Sistem wallet dengan REST API yang menjamin integritas data dan keamanan concurrency. Dibangun dengan Go, Echo framework, dan PostgreSQL.

## Cara Menjalankan

### Prasyarat
- Go 1.21+
- PostgreSQL 14+ (lokal atau via Docker)
- Docker & Docker Compose (opsional, untuk PostgreSQL)

### Setup Database

**Opsi 1: Docker Compose**
```bash
make docker-up
```

**Opsi 2: PostgreSQL Lokal**
```bash
# Buat database dan user
psql -U postgres -c "CREATE DATABASE wallet_db;"
psql -U postgres -c "CREATE USER wallet_user WITH PASSWORD 'wallet_pass';"
psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE wallet_db TO wallet_user;"
psql -U postgres -c "ALTER DATABASE wallet_db OWNER TO wallet_user;"
```

### Konfigurasi

Salin dan sesuaikan file environment:
```bash
cp .env.example .env
```

Variabel yang tersedia:
| Variable | Default | Keterangan |
|---|---|---|
| `DB_HOST` | `localhost` | Host PostgreSQL |
| `DB_PORT` | `5432` | Port PostgreSQL |
| `DB_USER` | `wallet_user` | Username database |
| `DB_PASSWORD` | `wallet_pass` | Password database |
| `DB_NAME` | `wallet_db` | Nama database |
| `DB_SSLMODE` | `disable` | SSL mode |
| `SERVER_PORT` | `8080` | Port server HTTP |

### Menjalankan Aplikasi

```bash
make run
# atau
go run cmd/server/main.go
```

Migrasi database dijalankan otomatis saat aplikasi dimulai.

### Menjalankan Tes

```bash
# Unit tests saja
make test-unit

# Integration tests (perlu PostgreSQL)
make test-integration

# Semua tes
make test
```

---

## Keputusan Desain & Alasannya

### 1. Representasi Uang: `BIGINT` (Satuan Terkecil)

Semua nilai uang disimpan sebagai `BIGINT` dalam satuan terkecil mata uang (misalnya, Rupiah utuh, bukan desimal). Ini menghilangkan masalah pembulatan floating-point sepenuhnya. API menerima dan mengembalikan nilai integer.

**Alasan**: Floating-point (`float64`) tidak bisa merepresentasikan semua bilangan desimal secara akurat (misalnya, `0.1 + 0.2 != 0.3`). Dalam domain keuangan, keakuratan nilai uang adalah keharusan mutlak.

### 2. Double-Entry Bookkeeping (Pembukuan Berpasangan)

Setiap mutasi saldo menghasilkan **dua entri ledger**: satu debit dan satu kredit. Untuk top-up, akun sistem (`SYSTEM`) bertindak sebagai counter-party.

| Operasi | Debit (dari) | Kredit (ke) |
|---|---|---|
| Top-up | Akun SYSTEM | Wallet user |
| Transfer | Wallet pengirim | Wallet penerima |
| Reversal | Kebalikan dari operasi asli | Kebalikan dari operasi asli |

**Alasan**: Double-entry menjamin bahwa setiap perpindahan uang selalu seimbang. Jika ada selisih, artinya ada bug — dan tes rekonsiliasi akan mendeteksinya.

### 3. Pessimistic Locking dengan Urutan Konsisten

Saat mutasi, wallet dikunci dengan `SELECT ... FOR UPDATE`. Untuk transfer yang melibatkan dua wallet, penguncian selalu dilakukan **dalam urutan ascending berdasarkan user ID**.

```
Transfer A→B: lock(min(A,B)), lock(max(A,B))
Transfer B→A: lock(min(A,B)), lock(max(A,B))  ← urutan sama!
```

**Alasan**: Tanpa urutan penguncian yang konsisten, dua transfer bersamaan (`A→B` dan `B→A`) bisa saling menunggu (deadlock). Dengan mengunci berdasarkan urutan ID, deadlock tidak mungkin terjadi.

### 4. Database-Level Safety Net

Constraint `CHECK (balance >= 0)` pada tabel `wallets` memastikan bahwa **bahkan jika ada bug di aplikasi**, PostgreSQL akan menolak saldo negatif. Ini adalah pertahanan berlapis (defense in depth).

### 5. Idempotency yang Tahan Balapan (Race-Proof)

Idempotency key disimpan dengan **UNIQUE constraint** pada tabel `transactions`. Mekanisme penanganannya:

1. **Fast path**: Cek apakah key sudah ada (di luar transaksi, tanpa lock). Jika ada, kembalikan response yang di-cache.
2. **Slow path**: Jika belum ada, jalankan operasi dalam transaksi database.
3. **Race handling**: Jika dua request identik tiba bersamaan dan keduanya melewati fast path, yang kalah akan mendapat error unique constraint violation. Request yang kalah akan rollback dan melakukan retry lookup ke tabel idempotency untuk mendapatkan response dari pemenang.

**Alasan**: `INSERT ... ON CONFLICT` pada level database menjamin atomisitas yang tidak bisa dicapai oleh lock di level aplikasi saja.

### 6. Append-Only Ledger dengan Reversal

Entri ledger **tidak pernah dihapus atau dimodifikasi**. Pembatalan transaksi dilakukan dengan membuat **entri lawan** (reversal), bukan menghapus catatan lama. Ini menjaga integritas audit trail.

Setiap entri ledger juga menyimpan `balance_after` sebagai snapshot saldo setelah entri tersebut, memungkinkan pembacaan cepat tanpa menghitung ulang dari awal.

### 7. Struktur Kode

Menggunakan arsitektur berlapis yang umum di Go:
- **Domain**: Entitas dan DTO murni (tanpa dependensi infrastruktur)
- **Repository**: Akses data (SQL queries, lock management)
- **Service**: Logika bisnis (validasi, orkestrasi transaksi)
- **Handler**: Penanganan HTTP (parsing request, formatting response)

**Alasan**: Pemisahan ini memudahkan testing (repository bisa di-mock untuk unit test) dan membuat kode lebih mudah dipahami.

---

## API Endpoints

### User

| Method | Path | Deskripsi |
|---|---|---|
| `POST` | `/api/v1/users` | Daftar user baru (otomatis buat wallet) |
| `GET` | `/api/v1/users/:id` | Lihat detail user |

### Wallet

| Method | Path | Deskripsi |
|---|---|---|
| `GET` | `/api/v1/wallets/:userId` | Lihat saldo wallet |
| `POST` | `/api/v1/wallets/:userId/topup` | Top-up saldo |
| `GET` | `/api/v1/wallets/:userId/mutations` | Lihat mutasi (paginasi + filter tanggal) |

### Transfer

| Method | Path | Deskripsi |
|---|---|---|
| `POST` | `/api/v1/transfers` | Transfer saldo antar user |
| `POST` | `/api/v1/transfers/:id/reverse` | Batalkan transaksi (reversal) |

### Sistem

| Method | Path | Deskripsi |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/api/v1/reconciliation` | Jalankan pemeriksaan konsistensi |

### Header Wajib

Semua operasi mutasi memerlukan header:
```
Idempotency-Key: <string unik>
```

### Contoh Penggunaan (curl)

```bash
# 1. Buat user
curl -s -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@example.com"}'

# 2. Buat user kedua
curl -s -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"bob","email":"bob@example.com"}'

# 3. Top-up (ganti <alice-id> dengan ID dari response)
curl -s -X POST http://localhost:8080/api/v1/wallets/<alice-id>/topup \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: topup-001" \
  -d '{"amount":100000}'

# 4. Transfer
curl -s -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: transfer-001" \
  -d '{"from_user_id":"<alice-id>","to_user_id":"<bob-id>","amount":50000}'

# 5. Lihat mutasi dengan paginasi dan filter tanggal
curl -s "http://localhost:8080/api/v1/wallets/<alice-id>/mutations?page=1&per_page=10&start_date=2026-01-01&end_date=2026-12-31"

# 6. Lihat saldo
curl -s http://localhost:8080/api/v1/wallets/<alice-id>

# 7. Reversal (ganti <txn-id> dengan transaction ID)
curl -s -X POST http://localhost:8080/api/v1/transfers/<txn-id>/reverse \
  -H "Idempotency-Key: reverse-001"

# 8. Rekonsiliasi
curl -s -X POST http://localhost:8080/api/v1/reconciliation
```

---

## Tes Otomatis

### Unit Tests
- Validasi tipe error dan HTTP status code
- Verifikasi pesan error yang deskriptif

### Integration Tests
| Test | Apa yang Diuji |
|---|---|
| `TestTopUp_Success` | Top-up menambah saldo dan membuat ledger entry |
| `TestTopUp_Idempotency` | Key yang sama tidak mengeksekusi dua kali |
| `TestTransfer_Success` | Transfer atomik (kedua wallet terupdate) |
| `TestTransfer_InsufficientBalance` | Transfer gagal jika saldo kurang |
| `TestTransfer_SelfTransfer` | Transfer ke diri sendiri ditolak |
| `TestTransfer_InvalidAmount` | Nominal nol/negatif ditolak |
| `TestTransfer_WalletNotFound` | Wallet tidak ditemukan → error jelas |
| `TestReversal_TopUp` | Reversal top-up mengembalikan saldo |
| `TestReversal_Transfer` | Reversal transfer mengembalikan kedua wallet |
| `TestReversal_AlreadyReversed` | Reversal ganda ditolak |
| `TestMutations_PaginationAndDateFilter` | Paginasi dan filter tanggal berfungsi |
| `TestReconciliation_BalanceMatchesMutations` | Saldo = jumlah seluruh entri ledger |
| `TestDoubleEntry_LedgerBalances` | Setiap transaksi memiliki debit & kredit seimbang |

### Concurrent Tests
| Test | Apa yang Diuji |
|---|---|
| `TestConcurrent_TransfersNoNegativeBalance` | 20 transfer bersamaan: tepat 10 berhasil, saldo ≥ 0 |
| `TestConcurrent_IdempotentTransfers` | 10 request identik: hanya 1 transaksi terjadi |
| `TestConcurrent_BidirectionalTransfers` | Transfer dua arah bersamaan: total uang konservatif |

---

## Batasan yang Disadari

1. **Migrasi database** dijalankan via file SQL langsung saat startup. Untuk production, sebaiknya gunakan tool migrasi seperti `golang-migrate` atau `goose` dengan versioning yang lebih baik.

2. **Akun SYSTEM** untuk top-up memiliki saldo nominal 0. Dalam production seharusnya ada mekanisme terpisah untuk tracking source of funds.

3. **Idempotency key expiry** (24 jam) diberlakukan di level SQL tapi belum ada job scheduler untuk membersihkan key yang sudah expired.

4. **Multi-currency**: Field `currency` sudah ada di tabel `wallets`, tapi validasi bahwa transfer hanya terjadi antara wallet dengan currency yang sama belum diimplementasi.

5. **Authentication/Authorization**: Tidak ada autentikasi. Dalam production, setiap endpoint harus memvalidasi bahwa user hanya bisa mengakses wallet miliknya sendiri.

6. **Rate limiting**: Belum ada pembatasan rate. Dalam production, diperlukan untuk mencegah abuse.

7. **Logging**: Menggunakan `log` standar. Untuk production, sebaiknya gunakan structured logging (misalnya `slog` atau `zerolog`).

---

## Matriks Kelengkapan

| Requirement | Level | Status |
|---|---|---|
| User punya satu wallet dengan saldo | Mandatory | ✅ |
| Top-up dan transfer | Mandatory | ✅ |
| Saldo tidak boleh minus + transfer atomik | Mandatory | ✅ |
| Double-entry ledger (debit/kredit) | Mandatory | ✅ |
| Mutasi dengan paginasi + filter tanggal | Mandatory | ✅ |
| Idempotency key | Mandatory | ✅ |
| Uang disimpan sebagai integer (satuan terkecil) | Nice to Have | ✅ |
| Rekonsiliasi saldo dari mutasi + tes | Nice to Have | ✅ |
| Pessimistic locking dengan urutan konsisten | Nice to Have | ✅ |
| Tes transfer bersamaan (no negatif/duplikat) | Nice to Have | ✅ |
| Validasi awal dengan pesan jelas | Nice to Have | ✅ |
| Idempotency tahan balapan (UNIQUE constraint) | Enhancement | ✅ |
| Reversal sebagai entri lawan | Enhancement | ✅ |
| Ledger append-only dengan snapshot saldo | Enhancement | ✅ |
| Pemeriksaan konsistensi (rekonsiliasi) | Enhancement | ✅ |
