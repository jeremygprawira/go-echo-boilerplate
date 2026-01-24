package service

import (
	"context"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/formatter"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/pkg/validator"
)

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
	// Enrich wide event with business context
	logger.EnrichContext(ctx, "operation", "user_create")

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
			logger.EnrichContext(ctx, "phone_format_error", err.Error())
			return nil, errorc.Error(errorc.ErrorInvalidInput, "Invalid phone number format")
		}
		phoneNumber = *formattedPhoneNumber
	}

	// Check if user already exists
	isUserExist, err := us.d.Repository.Postgre.User.CheckByEmailOrPhoneNumber(ctx, request.Email, phoneNumber)
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "DatabaseError",
			Code:      "USER_CHECK_FAILED",
			Message:   err.Error(),
			Retriable: true, // Database queries can be retried
		})
		return nil, errorc.Error(errorc.ErrorDatabase)
	}

	if isUserExist {
		logger.EnrichContextMap(ctx, map[string]any{
			"conflict_email": request.Email,
			"conflict_phone": phoneNumber,
		})
		return nil, errorc.Error(errorc.ErrorAlreadyExist, "User with the same email or phone number already exists")
	}

	// Generate unique account number
	accountNumber, err := generator.AccountNumber()
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "GenerationError",
			Code:      "ACCOUNT_NUMBER_GENERATION_FAILED",
			Message:   err.Error(),
			Retriable: false,
		})
		return nil, errorc.Error(errorc.ErrorInternalServer, "Failed to generate account number")
	}

	// Hash password
	hashedPassword, err := generator.Hash(request.Password)
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "HashError",
			Code:      "PASSWORD_HASH_FAILED",
			Message:   err.Error(),
			Retriable: false,
		})
		return nil, errorc.Error(errorc.ErrorInternalServer, "Failed to hash password")
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

	// Persist user to database
	err = us.d.Repository.Postgre.User.Create(ctx, user)
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "DatabaseError",
			Code:      "USER_CREATE_FAILED",
			Message:   err.Error(),
			Retriable: false, // Creation failures usually indicate constraint violations
		})
		return nil, errorc.Error(errorc.ErrorDatabase, "Failed to create user")
	}

	// Enrich wide event with success metrics
	logger.EnrichContextMap(ctx, map[string]any{
		"user_account_number": accountNumber,
		"user_has_email":      request.Email != "",
		"user_has_phone":      phoneNumber != "",
	})

	return user, nil
}

func (us *userService) GetTokens(ctx context.Context, request *models.GetUserTokenRequest) (*models.GetUserTokenResponse, error) {
	if request.PhoneNumber.Number != "" {
		formattedPhoneNumber, err := formatter.PhoneNumber(models.PhoneNumber{
			Number:      request.PhoneNumber.Number,
			CountryCode: request.PhoneNumber.CountryCode,
		})
		if err != nil {
			logger.EnrichContext(ctx, "phone_format_error", err.Error())
			return nil, errorc.Error(errorc.ErrorInvalidInput, "Invalid phone number format")
		}
		request.PhoneNumber.Number = *formattedPhoneNumber
	}

	user, err := us.d.Repository.Postgre.User.GetCredentialsByEmailOrPhoneNumber(ctx, request.Email, request.PhoneNumber.Number)
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "DatabaseError",
			Code:      "USER_GET_CREDENTIALS_FAILED",
			Message:   err.Error(),
			Retriable: false,
		})
		return nil, errorc.Error(errorc.ErrorDatabase, "Failed to get user credentials")
	}

	if user == nil {
		logger.EnrichContextMap(ctx, map[string]any{
			"email": request.Email,
			"phone": request.PhoneNumber.Number,
		})
		return nil, errorc.Error(errorc.ErrorDataNotFound, "User not found")
	}

	// Verify password
	match, err := validator.Hash(request.Password, user.Password)
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "VerificationError",
			Code:      "PASSWORD_VERIFICATION_FAILED",
			Message:   err.Error(),
			Retriable: false,
		})
		return nil, errorc.Error(errorc.ErrorInvalidInput, "Invalid password")
	}

	if !match {
		logger.EnrichContextMap(ctx, map[string]any{
			"email": request.Email,
			"phone": request.PhoneNumber.Number,
		})
		return nil, errorc.Error(errorc.ErrorInvalidInput, "Invalid password")
	}

	var tokens []models.Token

	// Generate tokens
	accessToken, err := generator.AccessToken(user, us.d.JWTConfig)
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "TokenGenerationError",
			Code:      "ACCESS_TOKEN_GENERATION_FAILED",
			Message:   err.Error(),
			Retriable: false,
		})
		return nil, errorc.Error(errorc.ErrorInternalServer, "Failed to generate access token")
	}

	tokens = append(tokens, models.Token{
		Type:      models.TYPE_ACCESS_TOKEN,
		Token:     accessToken.Token,
		ExpiredIn: accessToken.ExpiredIn,
	})

	refreshToken, err := generator.RefreshToken(user, us.d.JWTConfig)
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "TokenGenerationError",
			Code:      "REFRESH_TOKEN_GENERATION_FAILED",
			Message:   err.Error(),
			Retriable: false,
		})
		return nil, errorc.Error(errorc.ErrorInternalServer, "Failed to generate refresh token")
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
	user, err := us.d.Repository.Postgre.User.GetOneByAccountNumber(ctx, accountNumber)
	if err != nil {
		logger.SetErrorContext(ctx, &logger.ErrorContext{
			Type:      "DatabaseError",
			Code:      "USER_GET_FAILED",
			Message:   err.Error(),
			Retriable: false,
		})
		return nil, errorc.Error(errorc.ErrorDatabase, "Failed to get user")
	}

	if user == nil {
		logger.EnrichContextMap(ctx, map[string]any{
			"account_number": accountNumber,
		})
		return nil, errorc.Error(errorc.ErrorDataNotFound, "User not found")
	}

	return user, nil
}
