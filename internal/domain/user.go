package domain

import "time"

// User represents a registered user in the system.
type User struct {
	ID        string    `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Email     string    `json:"email" db:"email"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CreateUserRequest is the input for creating a new user.
type CreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=100"`
	Email    string `json:"email" validate:"required,email,max=255"`
}

// LoginRequest is the input for user authentication.
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
}

// AuthResponse is returned on successful login or registration.
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}
