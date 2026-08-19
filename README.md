# Wallet & Transaksi Saldo API

Sistem wallet dengan REST API yang menjamin integritas data, keamanan concurrency, pembukuan *double-entry ledger*, idempotency tahan balapan, autentikasi JWT, validasi `go-playground/validator`, *structured logging* `zerolog`, dan dokumentasi OpenAPI / Swagger UI. Dibangun menggunakan **Go (Echo Framework)** dan **PostgreSQL**.

---

## Cara Menjalankan

### Prasyarat
- **Docker & Docker Compose** (direkomendasikan untuk menjalankan seluruh aplikasi + database)
- **Go 1.24+** (jika ingin menjalankan secara lokal tanpa Docker)
- **PostgreSQL 14+** (jika menggunakan PostgreSQL lokal)

---

### ⚠️ Catatan Kompatibilitas Sistem Operasi
> [!NOTE]
> Perintah `make` (seperti `make docker-up`, `make run`, `make test`) dirancang untuk lingkungan **Linux**, **macOS**, atau **Windows Subsystem for Linux (WSL)** / Git Bash yang memiliki utilitas `make`.
> 
> Bagi pengguna **Windows (PowerShell / Command Prompt)**, gunakan perintah langsung (`docker compose`, `go run`, `go test`) yang telah disediakan pada panduan di bawah ini.

---

### Opsi 1: Menggunakan Docker Compose (Paling Mudah - App + Database)

Cukup jalankan satu perintah untuk membangun Docker Image aplikasi dan menjalankan container PostgreSQL & server API secara bersamaan:

#### 🐧 Linux / macOS / WSL (via Make):
```bash
make docker-up
```

#### 🪟 Windows (PowerShell / Command Prompt):
```powershell
docker compose up -d --build
```

Setelah container berjalan:
- **API Server**: `http://localhost:8080`
- **Swagger UI**: `http://localhost:8080/swagger/index.html`
- **Health Check**: `http://localhost:8080/health`

Untuk melihat log aplikasi:
- **Linux/macOS**: `make docker-logs`
- **Windows**: `docker compose logs -f app`

Untuk menghentikan container:
- **Linux/macOS**: `make docker-down`
- **Windows**: `docker compose down`

---

### Opsi 2: Menjalankan Secara Lokal (Go + Docker PostgreSQL)

Jika Anda ingin menjalankan server Go secara lokal di komputer host dan hanya menggunakan PostgreSQL di Docker:

1. **Jalankan PostgreSQL via Docker**:
   - **Linux/macOS**: `make docker-up` (atau `docker compose up -d postgres`)
   - **Windows**: `docker compose up -d postgres`

2. **Salin File Environment**:
   - **Linux/macOS**: `cp .env.example .env`
   - **Windows (PowerShell)**: `copy .env.example .env` (atau `cp .env.example .env` di Git Bash)

3. **Jalankan Aplikasi Go**:
   - **Linux/macOS**: `make run`
   - **Windows**: `go run cmd/server/main.go`

---

### Opsi 3: Menjalankan Tes Otomatis

- **Linux/macOS**:
  ```bash
  make test           # Jalankan semua unit & integration test
  make test-unit      # Unit test saja
  make test-integration # Integration test saja
  ```

- **Windows (PowerShell / Command Prompt)**:
  ```powershell
  # Unit tests saja (tanpa koneksi DB)
  go test ./tests/unit/... -v -count=1

  # Integration tests (memerlukan PostgreSQL berjalan)
  go test ./tests/integration/... -v -count=1 -timeout=120s

  # Semua tes
  go test ./... -v -count=1 -timeout=120s
  ```

---

## Konfigurasi (.env)

| Variable | Default | Keterangan |
|---|---|---|
| `DB_HOST` | `localhost` | Host PostgreSQL |
| `DB_PORT` | `5432` | Port PostgreSQL |
| `DB_USER` | `wallet_user` | Username database |
| `DB_PASSWORD` | `wallet_pass` | Password database |
| `DB_NAME` | `wallet_db` | Nama database |
| `DB_SSLMODE` | `disable` | SSL mode |
| `SERVER_PORT` | `8080` | Port server HTTP |
| `JWT_SECRET` | `secret` | Secret key untuk signing token JWT |

---

## Perintah Makefile (Linux / macOS / WSL)

| Perintah | Keterangan | Perintah Setara Windows |
|---|---|---|
| `make run` | Menjalankan aplikasi secara lokal | `go run cmd/server/main.go` |
| `make test` | Menjalankan seluruh unit & integration test | `go test ./... -v -count=1 -timeout=120s` |
| `make test-unit` | Menjalankan unit test saja | `go test ./tests/unit/... -v -count=1` |
| `make test-integration` | Menjalankan integration test | `go test ./tests/integration/... -v -count=1 -timeout=120s` |
| `make swagger` | Generasi ulang dokumen OpenAPI / Swagger UI | `swag init -g cmd/server/main.go` |
| `make docker-build` | Membangun Docker image `wallet-app:latest` | `docker build -t wallet-app:latest .` |
| `make docker-up` | Membangun & menjalankan container App + PostgreSQL | `docker compose up -d --build` |
| `make docker-down` | Menghentikan container Docker | `docker compose down` |
| `make docker-logs` | Melihat log container aplikasi | `docker compose logs -f app` |
| `make docker-reset` | Mereset database dan container dari awal | `docker compose down -v` lalu `docker compose up -d --build` |

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
Constraint `CHECK (balance >= 0)` pada tabel `wallets` memastikan bahwa **bahkan jika ada bug di aplikasi**, PostgreSQL akan menolak saldo negatif. Ini adalah pertahanan berlapis (*defense in depth*).

### 5. Idempotency yang Tahan Balapan (Race-Proof)
Idempotency key disimpan dengan **UNIQUE constraint** pada tabel `transactions`. Mekanisme penanganannya:
1. **Fast path**: Cek apakah key sudah ada (di luar transaksi, tanpa lock). Jika ada, kembalikan response yang di-cache.
2. **Slow path**: Jika belum ada, jalankan operasi dalam transaksi database.
3. **Race handling**: Jika dua request identik tiba bersamaan dan keduanya melewati fast path, yang kalah akan mendapat error unique constraint violation. Request yang kalah akan rollback dan melakukan retry lookup ke tabel idempotency untuk mendapatkan response dari pemenang.

**Alasan**: `INSERT ... ON CONFLICT` pada level database menjamin atomisitas yang tidak bisa dicapai oleh lock di level aplikasi saja.

### 6. Autentikasi JWT & Otorisasi Berbasis Claims Payload
- **Enkripsi Password**: Password di-hash menggunakan algoritma `bcrypt` pada saat registrasi dan login.
- **Payload JWT**: Token JWT menyimpan `user_id` di dalam payload claims.
- **Pembersihan URL Params**: Seluruh protected endpoint (`/api/v1/users/me`, `/api/v1/wallets`, `/api/v1/wallets/topup`, `/api/v1/wallets/mutations`, `/api/v1/transfers`) secara eksplisit mengekstrak `user_id` dari JWT token. User tidak dapat melakukan manipulasi atau melihat data wallet milik user lain.

### 7. Validasi Request (`go-playground/validator`)
Validasi input request dipisahkan secara tegas di layer HTTP/handler menggunakan library `go-playground/validator/v10` melalui adapter `CustomValidator` pada Echo (`e.Validator`). Layer `service` sepenuhnya bersih dari logika validasi struktur request dan berfokus pada aturan bisnis domain.

### 8. Structured Logging (`zerolog`)
Seluruh sistem logging dikonfigurasi menggunakan `rs/zerolog` untuk structured logging yang cepat, efisien, dan mendukung format JSON di lingkungan produksi serta format konsol berwarna di lingkungan pengembangan.

---

## API Endpoints

### Auth & User

| Method | Path | Akses | Deskripsi |
|---|---|---|---|
| `POST` | `/api/v1/users` | Publik | Daftar user baru (username, email, password; mengembalikan JWT token) |
| `POST` | `/api/v1/auth/login` | Publik | Login user (email, password; mengembalikan JWT token) |
| `GET` | `/api/v1/users/me` | Protected | Lihat detail user terautentikasi (berdasarkan JWT token) |

### Wallet & Transfer

| Method | Path | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/v1/wallets` | Protected | Lihat saldo wallet terautentikasi (berdasarkan JWT token) |
| `POST` | `/api/v1/wallets/topup` | Protected | Top-up saldo wallet terautentikasi (berdasarkan JWT token) |
| `GET` | `/api/v1/wallets/mutations` | Protected | Lihat mutasi ledger terautentikasi (berdasarkan JWT token) |
| `POST` | `/api/v1/transfers` | Protected | Transfer saldo ke `to_user_id` (pengirim diset dari JWT token) |
| `POST` | `/api/v1/transfers/:id/reverse` | Protected | Batalkan transaksi (reversal) |

### Sistem

| Method | Path | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/health` | Publik | Health check |
| `GET` | `/swagger/index.html` | Publik | Dokumentasi Interactive OpenAPI / Swagger UI |
| `POST` | `/api/v1/reconciliation` | Publik | Jalankan pemeriksaan konsistensi |

---

### Contoh Penggunaan (curl)

```bash
# 1. Buat user Alice
ALICE_RESP=$(curl -s -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@example.com","password":"password123"}')
ALICE_TOKEN=$(echo $ALICE_RESP | jq -r '.data.token')
ALICE_ID=$(echo $ALICE_RESP | jq -r '.data.user.id')

# 2. Buat user Bob
BOB_RESP=$(curl -s -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"bob","email":"bob@example.com","password":"password123"}')
BOB_ID=$(echo $BOB_RESP | jq -r '.data.user.id')

# 3. Login Bob menggunakan Email dan Password
BOB_LOGIN_RESP=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@example.com","password":"password123"}')
BOB_TOKEN=$(echo $BOB_LOGIN_RESP | jq -r '.data.token')

# 4. Top-up Alice (menggunakan Bearer Token Alice)
curl -s -X POST http://localhost:8080/api/v1/wallets/topup \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: topup-001" \
  -d '{"amount":100000}'

# 5. Transfer Alice -> Bob (Pengirim otomatis diset dari payload JWT Alice)
curl -s -X POST http://localhost:8080/api/v1/transfers \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: transfer-001" \
  -d "{\"to_user_id\":\"$BOB_ID\",\"amount\":50000}"

# 6. Lihat Profil Bob (JWT Token Bob)
curl -s http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer $BOB_TOKEN"

# 7. Lihat Saldo Bob (JWT Token Bob)
curl -s http://localhost:8080/api/v1/wallets \
  -H "Authorization: Bearer $BOB_TOKEN"
```

---

## Batasan yang Disadari & Trade-offs (Alasan & Rationale)

1. **Migrasi Database Inline SQL saat Startup**
   - *Alasan*: Menjalankan file SQL migrasi `001_init.sql` secara otomatis saat server berjalan menyederhanakan *deployment* dan *testing* (*zero external migration CLI dependency*). Aplikasi/container dapat langsung dijalankan tanpa perlu memasang CLI terpisah.
   - *Rekomendasi Production*: Pada skala *enterprise*, disarankan menggunakan *migration tool* berversi terpisah seperti `golang-migrate` atau `goose` agar eksekusi migrasi decoupled dari startup server HTTP.

2. **Akun Virtual `SYSTEM` untuk Top-Up**
   - *Alasan*: Menggunakan akun virtual `00000000-0000-0000-0000-000000000000` sebagai *counter-party* debit saat transaksi *top-up* untuk memenuhi prinsip akuntansi berpasangan (*double-entry bookkeeping*). Akun ini bertindak sebagai entitas virtual penerbit saldo.
   - *Rekomendasi Production*: Dalam sistem perbankan/e-wallet riil, entri debit top-up dihubungkan ke *settlement/nostro account* bank mitra dengan proses rekonsiliasi kas fisik.

3. **Pembersihan Idempotency Key Expired (24 Jam)**
   - *Alasan*: Pengecekan idempotensi dilakukan secara langsung pada tabel database `idempotency_keys` berdasarkan *UNIQUE constraint* dan waktu pembuatan untuk menjamin atomisitas tanpa menambah dependensi memori tambahan.
   - *Rekomendasi Production*: Ditambahkan *cron job* / `pg_cron` atau *background cleanup worker* (atau menggunakan Redis TTL) untuk secara berkala menghapus baris kunci yang sudah melewati batas 24 jam agar ukuran tabel tetap efisien.

4. **Cakupan Multi-Currency**
   - *Alasan*: Skema tabel `wallets` sudah menyediakan kolom `currency` (default `'IDR'`) untuk memfasilitasi pengembangan *multi-currency* (*future-proofing*).
   - *Rekomendasi Production*: Dalam scope studi kasus ini, transaksi dibatasi pada mata uang seragam (IDR) untuk menghindari kompleksitas konversi kurs valas (*exchange rate floating*).

5. **Cakupan Rate Limiting**
   - *Alasan*: Fokus utama pengerjaan difokuskan pada integritas data, atomisitas pembukuan ledger, *pessimistic locking*, dan jaminan *race-condition safety* yang diuji intensif pada *concurrency test*.
   - *Rekomendasi Production*: Penerapan middleware *Rate Limiting* (misal token bucket via Redis/API Gateway) disarankan untuk mencegah ancaman DoS / brute-force.

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
| Validasi awal dengan pesan jelas (`go-playground/validator`) | Nice to Have | ✅ |
| Autentikasi JWT & Otorisasi Berbasis Claims Payload | Nice to Have | ✅ |
| Interactive OpenAPI / Swagger UI (`/swagger/index.html`) | Nice to Have | ✅ |
| Multi-stage Dockerfile & Containerization (`docker compose`) | Nice to Have | ✅ |
| Idempotency tahan balapan (UNIQUE constraint) | Enhancement | ✅ |
| Reversal sebagai entri lawan | Enhancement | ✅ |
| Ledger append-only dengan snapshot saldo | Enhancement | ✅ |
| Pemeriksaan konsistensi (rekonsiliasi) | Enhancement | ✅ |
