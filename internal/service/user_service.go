package service

import (
	"context"
	"errors"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/formatter"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/pkg/validator"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgUniqueViolation is the PostgreSQL error code for a UNIQUE constraint violation.
// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
const pgUniqueViolation = "23505"

// dummyPasswordHash is a valid bcrypt hash compared against on the
// user-not-found path so login timing does not reveal whether an account
// exists. It corresponds to no real password.
const dummyPasswordHash = "$2a$12$BOZVmY4H76pfJnkVfAJEk.m5t0QcXHgphRl4wrKGSl8F7A5PnQRC2"

type UserService interface {
	Create(ctx context.Context, request *models.CreateUserRequest) (*models.User, error)
	GetTokens(ctx context.Context, request *models.GetUserTokenRequest) (*models.GetUserTokenResponse, error)
	GetByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error)
}

type userService struct {
	d *Dependencies
}

func NewUserService(d *Dependencies) UserService {
	return &userService{d: d}
}

func (us *userService) Create(ctx context.Context, request *models.CreateUserRequest) (*models.User, error) {
	// Enrich wide event with all known request fields upfront in a single grouped call.
	logger.AddToKey(ctx, "user", map[string]any{
		"operation":          "user.create",
		"raw_phone_number":   request.PhoneNumber.Number,
		"phone_country_code": request.PhoneNumber.CountryCode,
		"email":              request.Email,
	})

	// Process phone number formatting if provided
	phoneNumber := request.PhoneNumber.Number
	phoneCountryCode := request.PhoneNumber.CountryCode

	// Format phone number to international format if provided
	if phoneNumber != "" {
		if phoneCountryCode == "" {
			phoneCountryCode = "ID"
		}
		formattedPhoneNumber, err := formatter.PhoneNumber(models.PhoneNumber{
			Number:      phoneNumber,
			CountryCode: phoneCountryCode,
		})
		if err != nil {
			return nil, errorc.Error(errorc.ErrorInvalidInput, err, "Invalid phone number format")
		}
		phoneNumber = *formattedPhoneNumber
	}

	logger.AddToKey(ctx, "user", "formatted_phone_number", phoneNumber)

	// Generate unique account number
	accountNumber, err := generator.AccountNumber()
	if err != nil {
		return nil, errorc.Error(errorc.ErrorInternalServer, err, "Failed to generate account number")
	}

	// Hash password
	hashedPassword, err := generator.Hash(request.Password)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorInternalServer, err, "Failed to hash password")
	}

	var emailPtr *string
	if request.Email != "" {
		emailPtr = &request.Email
	}

	var phonePtr *string
	if phoneNumber != "" {
		phonePtr = &phoneNumber
	}

	// Create user model
	user := &models.User{
		AccountNumber:    accountNumber,
		Name:             request.Name,
		Email:            emailPtr,
		Password:         hashedPassword, // Use local variable, don't mutate input
		PhoneNumber:      phonePtr,
		PhoneCountryCode: phoneCountryCode,
		// CreatedAt and UpdatedAt will be set by database defaults
	}

	logger.AddToKey(ctx, "user", map[string]any{
		"account_number": accountNumber,
	})

	// Persist user to database
	err = us.d.Repository.User.Create(ctx, user)
	if err != nil {
		logger.AddToKey(ctx, "user", "is_inserted_to_db", false)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return nil, errorc.Error(errorc.ErrorAlreadyExist, err, "User already exists with that email or phone number")
		}
		return nil, errorc.Error(errorc.ErrorDatabase, err, "Failed to create user")
	}

	return user, nil
}

func (us *userService) GetTokens(ctx context.Context, request *models.GetUserTokenRequest) (*models.GetUserTokenResponse, error) {
	logger.AddToKey(ctx, "user", map[string]any{
		"operation": "user.get_tokens",
		"email":     request.Email,
		"phone":     request.PhoneNumber.Number,
	})

	if request.PhoneNumber.Number != "" {
		formattedPhoneNumber, err := formatter.PhoneNumber(models.PhoneNumber{
			Number:      request.PhoneNumber.Number,
			CountryCode: request.PhoneNumber.CountryCode,
		})
		if err != nil {
			return nil, errorc.Error(errorc.ErrorInvalidInput, err, "Invalid phone number format")
		}
		request.PhoneNumber.Number = *formattedPhoneNumber
		logger.AddToKey(ctx, "user", "formatted_phone", request.PhoneNumber.Number)
	}

	user, err := us.d.Repository.User.GetCredentialsByEmailOrPhoneNumber(ctx, request.Email, request.PhoneNumber.Number)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorDatabase, err, "Failed to get user credentials")
	}

	// Always run a bcrypt comparison — even when the user does not exist — so an
	// attacker cannot distinguish "no such account" from "wrong password" by
	// response message or timing. Both paths return one generic error.
	storedHash := dummyPasswordHash
	if user != nil {
		storedHash = user.Password
	}

	match, err := validator.Hash(request.Password, storedHash)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorInternalServer, err, "Failed to verify credentials")
	}

	if user == nil || !match {
		return nil, errorc.Error(errorc.ErrorUnauthorized, "invalid credentials")
	}

	var tokens []models.Token

	// Generate tokens
	accessToken, err := generator.AccessToken(user, us.d.JWTConfig)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorInternalServer, err, "Failed to generate access token")
	}

	tokens = append(tokens, models.Token{
		Type:      models.TYPE_ACCESS_TOKEN,
		Token:     accessToken.Token,
		ExpiredIn: accessToken.ExpiredIn,
	})

	refreshToken, err := generator.RefreshToken(user, us.d.JWTConfig)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorInternalServer, err, "Failed to generate refresh token")
	}

	tokens = append(tokens, models.Token{
		Type:      models.TYPE_REFRESH_TOKEN,
		Token:     refreshToken.Token,
		ExpiredIn: refreshToken.ExpiredIn,
	})

	email := ""
	if user.Email != nil {
		email = *user.Email
	}

	userPhoneNumber := ""
	if user.PhoneNumber != nil {
		userPhoneNumber = *user.PhoneNumber
	}

	return &models.GetUserTokenResponse{
		Type:          models.TYPE_USER,
		AccountNumber: user.AccountNumber,
		Name:          user.Name,
		Email:         email,
		PhoneNumber:   models.PhoneNumber{Number: userPhoneNumber, CountryCode: user.PhoneCountryCode},
		Tokens:        tokens,
	}, nil
}

func (us *userService) GetByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error) {
	logger.AddToKey(ctx, "user", map[string]any{
		"operation":      "user.get_by_account_number",
		"account_number": accountNumber,
	})

	user, err := us.d.Repository.User.GetOneByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorDatabase, err, "Failed to get user")
	}

	if user == nil {
		return nil, errorc.Error(errorc.ErrorDataNotFound, "User not found")
	}

	return user, nil
}
