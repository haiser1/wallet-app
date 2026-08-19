package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
	"test-teknis/internal/middleware"
	"test-teknis/internal/service"
)

// UserHandler handles HTTP requests for user operations.
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// CreateUser handles POST /api/v1/users (Public)
// @Summary Register a new user
// @Description Create a new user account and automatically generate an initial wallet & JWT token
// @Tags User
// @Accept json
// @Produce json
// @Param request body domain.CreateUserRequest true "User Registration Info"
// @Success 201 {object} map[string]domain.AuthResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 409 {object} errors.ErrorResponse
// @Router /api/v1/users [post]
func (h *UserHandler) CreateUser(c echo.Context) error {
	var req domain.CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return middleware.RespondError(c, appErrors.NewValidationError("invalid request body"))
	}

	if err := c.Validate(&req); err != nil {
		return middleware.RespondError(c, err)
	}

	authResp, err := h.userService.CreateUser(c.Request().Context(), req)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"data": authResp,
	})
}

// Login handles POST /api/v1/auth/login (Public)
// @Summary Login user
// @Description Authenticate user by username and return JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body domain.LoginRequest true "Login Request"
// @Success 200 {object} map[string]domain.AuthResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *UserHandler) Login(c echo.Context) error {
	var req domain.LoginRequest
	if err := c.Bind(&req); err != nil {
		return middleware.RespondError(c, appErrors.NewValidationError("invalid request body"))
	}

	if err := c.Validate(&req); err != nil {
		return middleware.RespondError(c, err)
	}

	authResp, err := h.userService.Login(c.Request().Context(), req)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": authResp,
	})
}

// GetUserProfile handles GET /api/v1/users/me (Protected - returns profile of current authenticated user)
// @Summary Get current user profile
// @Description Retrieve profile of the authenticated user using ID stored in JWT token
// @Tags User
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]domain.User
// @Failure 401 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Router /api/v1/users/me [get]
func (h *UserHandler) GetUserProfile(c echo.Context) error {
	authUserID, err := middleware.GetAuthenticatedUserID(c)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	user, err := h.userService.GetUser(c.Request().Context(), authUserID)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": user,
	})
}
