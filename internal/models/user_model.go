package models

import (
	"database/sql"
	"time"
)

var TYPE_USER = "user"

type User struct {
	ID               int          `json:"id"`
	AccountNumber    string       `json:"account_number"`
	Name             string       `json:"name"`
	Email            *string      `json:"email"`
	PhoneNumber      *string      `json:"phone_number"`
	PhoneCountryCode string       `json:"phone_country_code"`
	Password         string       `json:"-"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	DeletedAt        sql.NullTime `json:"deleted_at"`
}

type (
	CreateUserRequest struct {
		Name        string      `json:"name" validate:"required" example:"John Doe"`
		Email       string      `json:"email" validate:"required_without=PhoneNumber,omitempty,emailFormat" example:"john.doe@example.com"`
		PhoneNumber PhoneNumber `json:"phoneNumber" validate:"required_without=Email,omitempty"`
		Password    string      `json:"password" validate:"required" example:"password123"`
	}

	CreateUserResponse struct {
		Type          string      `json:"type" example:"user"`
		AccountNumber string      `json:"accountNumber" example:"1234567890"`
		Name          string      `json:"name" example:"John Doe"`
		Email         string      `json:"email" example:"john.doe@example.com"`
		PhoneNumber   PhoneNumber `json:"phoneNumber"`
		CreatedAt     time.Time   `json:"createdAt" example:"2026-01-24T15:57:37+07:00"`
		UpdatedAt     time.Time   `json:"updatedAt" example:"2026-01-24T15:57:37+07:00"`
	}
)

func (u *User) CreateUserResponse() *CreateUserResponse {
	email := ""
	if u.Email != nil {
		email = *u.Email
	}

	phoneNumber := ""
	if u.PhoneNumber != nil {
		phoneNumber = *u.PhoneNumber
	}

	return &CreateUserResponse{
		Type:          TYPE_USER,
		AccountNumber: u.AccountNumber,
		Name:          u.Name,
		Email:         email,
		PhoneNumber:   PhoneNumber{Number: phoneNumber, CountryCode: u.PhoneCountryCode},
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

type (
	GetUserTokenRequest struct {
		Email       string      `json:"email" validate:"required_without=PhoneNumber,omitempty,emailFormat" example:"john.doe@example.com"`
		PhoneNumber PhoneNumber `json:"phoneNumber" validate:"required_without=Email,omitempty"`
		Password    string      `json:"password" validate:"required" example:"password123"`
	}

	GetUserTokenResponse struct {
		Type          string      `json:"type" example:"user"`
		AccountNumber string      `json:"accountNumber" example:"1234567890"`
		Name          string      `json:"name" example:"John Doe"`
		Email         string      `json:"email" example:"john.doe@example.com"`
		PhoneNumber   PhoneNumber `json:"phoneNumber"`
		Tokens        []Token     `json:"tokens"`
	}

	// RefreshTokenRequest carries the refresh token in the body. The field is
	// optional here because the handler also accepts the token from the
	// refresh_token cookie set at login — whichever is present is used, body
	// taking precedence. Presence is enforced in the handler, not via a
	// struct tag, since it depends on the cookie too.
	RefreshTokenRequest struct {
		RefreshToken string `json:"refreshToken,omitempty"`
	}

	// LogoutRequest optionally carries the refresh token to revoke in the
	// body, for clients that don't rely on the refresh_token cookie set at
	// login. Same body-or-cookie resolution as RefreshTokenRequest.
	LogoutRequest struct {
		RefreshToken string `json:"refreshToken,omitempty"`
	}
)

type (
	GetUserByAccountNumberResponse struct {
		Type          string      `json:"type" example:"user"`
		AccountNumber string      `json:"accountNumber" example:"1234567890"`
		Name          string      `json:"name" example:"John Doe"`
		Email         string      `json:"email" example:"john.doe@example.com"`
		PhoneNumber   PhoneNumber `json:"phoneNumber"`
		CreatedAt     time.Time   `json:"createdAt" example:"2026-01-24T15:57:37+07:00"`
		UpdatedAt     time.Time   `json:"updatedAt" example:"2026-01-24T15:57:37+07:00"`
	}
)

func (u *User) GetUserByAccountNumberResponse() *GetUserByAccountNumberResponse {
	email := ""
	if u.Email != nil {
		email = *u.Email
	}

	phoneNumber := ""
	if u.PhoneNumber != nil {
		phoneNumber = *u.PhoneNumber
	}

	return &GetUserByAccountNumberResponse{
		Type:          TYPE_USER,
		AccountNumber: u.AccountNumber,
		Name:          u.Name,
		Email:         email,
		PhoneNumber: PhoneNumber{
			Number:      phoneNumber,
			CountryCode: u.PhoneCountryCode,
		},
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
