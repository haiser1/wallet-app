package service

import (
	"context"
	"fmt"

	"test-teknis/internal/auth"
	"test-teknis/internal/domain"
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

// CreateUser creates a new user with their wallet and returns user + JWT token.
func (s *UserService) CreateUser(ctx context.Context, req domain.CreateUserRequest) (*domain.AuthResponse, error) {
	user, err := s.userRepo.Create(ctx, req)
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

// Login authenticates a user by username and generates a JWT token containing user_id.
func (s *UserService) Login(ctx context.Context, req domain.LoginRequest) (*domain.AuthResponse, error) {
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
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

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, id)
}
