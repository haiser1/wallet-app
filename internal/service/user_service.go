package service

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"test-teknis/internal/auth"
	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
	"test-teknis/internal/repository"
)

// UserService handles user-related business logic.
type UserService struct {
	userRepo  *repository.UserRepository
	jwtSecret string
}

// NewUserService creates a new UserService.
func NewUserService(userRepo *repository.UserRepository, jwtSecret string) *UserService {
	return &UserService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

// CreateUser creates a new user with hashed password & wallet, returning user + JWT token.
func (s *UserService) CreateUser(ctx context.Context, req domain.CreateUserRequest) (*domain.AuthResponse, error) {
	// Hash password using bcrypt
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.Create(ctx, req, string(passwordHash))
	if err != nil {
		return nil, err
	}

	token, err := auth.GenerateToken(user.ID, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &domain.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// Login authenticates a user by email & password and generates a JWT token containing user_id.
func (s *UserService) Login(ctx context.Context, req domain.LoginRequest) (*domain.AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if err == appErrors.ErrUserNotFound {
			return nil, appErrors.ErrInvalidCredentials
		}
		return nil, err
	}

	// Compare bcrypt password hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, appErrors.ErrInvalidCredentials
	}

	token, err := auth.GenerateToken(user.ID, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &domain.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, id)
}
