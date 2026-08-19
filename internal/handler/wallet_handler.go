package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
	"test-teknis/internal/middleware"
	"test-teknis/internal/service"
)

// WalletHandler handles HTTP requests for wallet operations.
type WalletHandler struct {
	walletService *service.WalletService
}

// NewWalletHandler creates a new WalletHandler.
func NewWalletHandler(walletService *service.WalletService) *WalletHandler {
	return &WalletHandler{walletService: walletService}
}

// RegisterRoutes registers wallet-related routes on the Echo instance.
func (h *WalletHandler) RegisterRoutes(e *echo.Echo, protectedGroup *echo.Group) {
	// Protected endpoints (require JWT auth & ownership validation)
	protectedGroup.GET("/wallets/:userId", h.GetBalance)
	protectedGroup.POST("/wallets/:userId/topup", h.TopUp)
	protectedGroup.GET("/wallets/:userId/mutations", h.GetMutations)

	protectedGroup.POST("/transfers", h.Transfer)
	protectedGroup.POST("/transfers/:id/reverse", h.ReverseTransaction)

	e.POST("/api/v1/reconciliation", h.Reconcile)
}

// GetBalance handles GET /api/v1/wallets/:userId
// @Summary Get wallet balance
// @Description Get current wallet balance for the authenticated user
// @Tags Wallet
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]domain.WalletBalanceResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 403 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Router /api/v1/wallets/{userId} [get]
func (h *WalletHandler) GetBalance(c echo.Context) error {
	userID := c.Param("userId")
	if userID == "" {
		return middleware.RespondError(c, appErrors.NewValidationError("user id is required"))
	}
	if userID == domain.SystemWalletID {
		return middleware.RespondError(c, appErrors.ErrSystemWallet)
	}

	authUserID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	if authUserID != userID {
		return middleware.RespondError(c, appErrors.ErrForbidden)
	}

	resp, err := h.walletService.GetBalance(c.Request().Context(), userID)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": resp,
	})
}

// TopUp handles POST /api/v1/wallets/:userId/topup
// @Summary Top-up wallet
// @Description Deposit funds into the authenticated user's wallet
// @Tags Wallet
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Param Idempotency-Key header string true "Idempotency Key"
// @Param request body domain.TopUpRequest true "TopUp Info"
// @Success 200 {object} map[string]domain.TopUpResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 403 {object} errors.ErrorResponse
// @Router /api/v1/wallets/{userId}/topup [post]
func (h *WalletHandler) TopUp(c echo.Context) error {
	userID := c.Param("userId")
	if userID == "" {
		return middleware.RespondError(c, appErrors.NewValidationError("user id is required"))
	}
	if userID == domain.SystemWalletID {
		return middleware.RespondError(c, appErrors.ErrSystemWallet)
	}

	authUserID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	if authUserID != userID {
		return middleware.RespondError(c, appErrors.ErrForbidden)
	}

	var req domain.TopUpRequest
	if err := c.Bind(&req); err != nil {
		return middleware.RespondError(c, appErrors.NewValidationError("invalid request body"))
	}

	req.IdempotencyKey = c.Request().Header.Get("Idempotency-Key")

	if err := c.Validate(&req); err != nil {
		return middleware.RespondError(c, err)
	}

	resp, err := h.walletService.TopUp(c.Request().Context(), userID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": resp,
	})
}

// Transfer handles POST /api/v1/transfers
// @Summary Transfer funds
// @Description Transfer funds between wallets. Sender MUST be the authenticated user.
// @Tags Transfer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string true "Idempotency Key"
// @Param request body domain.TransferRequest true "Transfer Request"
// @Success 200 {object} map[string]domain.TransferResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 403 {object} errors.ErrorResponse
// @Router /api/v1/transfers [post]
func (h *WalletHandler) Transfer(c echo.Context) error {
	authUserID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	var req domain.TransferRequest
	if err := c.Bind(&req); err != nil {
		return middleware.RespondError(c, appErrors.NewValidationError("invalid request body"))
	}

	req.IdempotencyKey = c.Request().Header.Get("Idempotency-Key")

	if err := c.Validate(&req); err != nil {
		return middleware.RespondError(c, err)
	}

	// Ownership check: Sender MUST be the authenticated user!
	if req.FromUserID != authUserID {
		return middleware.RespondError(c, appErrors.ErrForbidden)
	}

	resp, err := h.walletService.Transfer(c.Request().Context(), req)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": resp,
	})
}

// ReverseTransaction handles POST /api/v1/transfers/:id/reverse
// @Summary Reverse transaction
// @Description Reverse a completed top-up or transfer transaction
// @Tags Transfer
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Param Idempotency-Key header string true "Idempotency Key"
// @Success 200 {object} map[string]domain.ReverseResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 409 {object} errors.ErrorResponse
// @Router /api/v1/transfers/{id}/reverse [post]
func (h *WalletHandler) ReverseTransaction(c echo.Context) error {
	txnID := c.Param("id")
	if txnID == "" {
		return middleware.RespondError(c, appErrors.NewValidationError("transaction id is required"))
	}

	_, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	var req domain.ReverseRequest
	req.IdempotencyKey = c.Request().Header.Get("Idempotency-Key")

	if err := c.Validate(&req); err != nil {
		return middleware.RespondError(c, err)
	}

	resp, err := h.walletService.Reverse(c.Request().Context(), txnID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": resp,
	})
}

// GetMutations handles GET /api/v1/wallets/:userId/mutations
// @Summary Get ledger mutations
// @Description Get paginated ledger entry mutations for the authenticated user
// @Tags Wallet
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Param page query int false "Page number (default: 1)"
// @Param per_page query int false "Items per page (default: 20, max: 100)"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Success 200 {object} domain.PaginatedMutations
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 403 {object} errors.ErrorResponse
// @Router /api/v1/wallets/{userId}/mutations [get]
func (h *WalletHandler) GetMutations(c echo.Context) error {
	userID := c.Param("userId")
	if userID == "" {
		return middleware.RespondError(c, appErrors.NewValidationError("user id is required"))
	}
	if userID == domain.SystemWalletID {
		return middleware.RespondError(c, appErrors.ErrSystemWallet)
	}

	authUserID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	if authUserID != userID {
		return middleware.RespondError(c, appErrors.ErrForbidden)
	}

	query := domain.MutationQuery{}

	// Parse pagination
	if p := c.QueryParam("page"); p != "" {
		page, err := strconv.Atoi(p)
		if err != nil || page < 1 {
			return middleware.RespondError(c, appErrors.NewValidationError("page must be a positive integer"))
		}
		query.Page = page
	}
	if pp := c.QueryParam("per_page"); pp != "" {
		perPage, err := strconv.Atoi(pp)
		if err != nil || perPage < 1 {
			return middleware.RespondError(c, appErrors.NewValidationError("per_page must be a positive integer"))
		}
		query.PerPage = perPage
	}

	// Parse date filters
	if sd := c.QueryParam("start_date"); sd != "" {
		t, err := time.Parse("2006-01-02", sd)
		if err != nil {
			return middleware.RespondError(c, appErrors.NewValidationError("start_date must be in YYYY-MM-DD format"))
		}
		query.StartDate = &t
	}
	if ed := c.QueryParam("end_date"); ed != "" {
		t, err := time.Parse("2006-01-02", ed)
		if err != nil {
			return middleware.RespondError(c, appErrors.NewValidationError("end_date must be in YYYY-MM-DD format"))
		}
		// Set end_date to end of day
		endOfDay := t.Add(24*time.Hour - time.Nanosecond)
		query.EndDate = &endOfDay
	}

	resp, err := h.walletService.GetMutations(c.Request().Context(), userID, query)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusOK, resp)
}

// Reconcile handles POST /api/v1/reconciliation
// @Summary System reconciliation
// @Description Run system-wide balance reconciliation check
// @Tags System
// @Produce json
// @Success 200 {object} map[string]domain.ReconciliationReport
// @Router /api/v1/reconciliation [post]
func (h *WalletHandler) Reconcile(c echo.Context) error {
	report, err := h.walletService.Reconcile(c.Request().Context())
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": report,
	})
}
