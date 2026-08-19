package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
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

// RegisterRoutes registers user-related routes on the Echo group.
func (h *UserHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.CreateUser)
	g.GET("/:id", h.GetUser)
}

// CreateUser handles POST /api/v1/users
func (h *UserHandler) CreateUser(c echo.Context) error {
	var req domain.CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, appErrors.NewValidationError("invalid request body"))
	}

	if err := c.Validate(&req); err != nil {
		return respondError(c, err)
	}

	user, err := h.userService.CreateUser(c.Request().Context(), req)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"data": user,
	})
}

// GetUser handles GET /api/v1/users/:id
func (h *UserHandler) GetUser(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return respondError(c, appErrors.NewValidationError("user id is required"))
	}

	user, err := h.userService.GetUser(c.Request().Context(), id)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": user,
	})
}
