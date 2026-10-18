package service

import (
	"context"
	"errors"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/formatter"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/validator"
	"time"

	"github.com/jeremygprawira/herr"
	"github.com/jeremygprawira/wlog"
)

// dummyPasswordHash is a valid bcrypt hash compared against on the
// user-not-found path so login timing does not reveal whether an account
// exists. It corresponds to no real password.
//
//nolint:gosec // G101: not a credential — a fixed decoy hash for constant-time comparison, not a secret.
const dummyPasswordHash = "$2a$12$BOZVmY4H76pfJnkVfAJEk.m5t0QcXHgphRl4wrKGSl8F7A5PnQRC2" //#nosec G101 -- decoy bcrypt hash for timing-safe login, not a real credential

type userStore interface {
	CreateUser(ctx context.Context, user *models.User) error
	UserCredentials(ctx context.Context, email string, phoneNumber string) (*models.User, error)
	UserByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error)
	UserByID(ctx context.Context, id int) (*models.User, error)
	RevokeToken(ctx context.Context, jti string, expiresAt time.Time) (bool, error)
}

type UserService struct {
	store userStore
	jwt   *jwtc.Configuration
}

func NewUserService(store userStore, jwt *jwtc.Configuration) *UserService {
	return &UserService{store: store, jwt: jwt}
}

func (us *UserService) Create(ctx context.Context, request *models.CreateUserRequest) (*models.User, error) {
	// Enrich wide event with all known request fields upfront in a single grouped call.
	wlog.SetGroup(ctx, "user", map[string]any{
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
			return nil, errorc.InvalidInput.New().Public(herr.Message("Invalid phone number format")).Wrap(err)
		}
		phoneNumber = *formattedPhoneNumber
	}

	wlog.SetGroup(ctx, "user", "formatted_phone_number", phoneNumber)

	// Generate unique account number
	accountNumber, err := generator.AccountNumber()
	if err != nil {
		return nil, errorc.Internal.New().Internal("failed to generate account number").Wrap(err)
	}

	if err := validator.PasswordWithinBcryptLimit(request.Password); err != nil {
		return nil, errorc.InvalidInput.New().Public(herr.Message("Password too long")).Wrap(err)
	}

	// Hash password
	hashedPassword, err := generator.Hash(request.Password)
	if err != nil {
		return nil, errorc.Internal.New().Internal("failed to hash password").Wrap(err)
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

	wlog.SetGroup(ctx, "user", map[string]any{
		"account_number": accountNumber,
	})

	// Persist user to database
	err = us.store.CreateUser(ctx, user)
	if err != nil {
		wlog.SetGroup(ctx, "user", "is_inserted_to_db", false)
		if errors.Is(err, models.ErrConflict) {
			return nil, errorc.AlreadyExists.New().Public(herr.Message("User already exists with that email or phone number")).Wrap(err)
		}
		return nil, errorc.Database.New().Internal("failed to create user").Wrap(err)
	}

	return user, nil
}

func (us *UserService) GetTokens(ctx context.Context, request *models.GetUserTokenRequest) (*models.GetUserTokenResponse, error) {
	wlog.SetGroup(ctx, "user", map[string]any{
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
			return nil, errorc.InvalidInput.New().Public(herr.Message("Invalid phone number format")).Wrap(err)
		}
		request.PhoneNumber.Number = *formattedPhoneNumber
		wlog.SetGroup(ctx, "user", "formatted_phone", request.PhoneNumber.Number)
	}

	user, err := us.store.UserCredentials(ctx, request.Email, request.PhoneNumber.Number)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, errorc.Database.New().Internal("failed to get user credentials").Wrap(err)
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
		return nil, errorc.Internal.New().Internal("failed to verify credentials").Wrap(err)
	}

	if user == nil || !match {
		return nil, errorc.Unauthorized.New().Public(herr.Message("invalid credentials"))
	}

	return us.issueTokens(user)
}

// issueTokens generates a fresh access+refresh pair for user and shapes the
// GetUserTokenResponse. Shared by GetTokens (login) and RefreshTokens.
func (us *UserService) issueTokens(user *models.User) (*models.GetUserTokenResponse, error) {
	var tokens []models.Token

	accessToken, err := jwtc.SignAccessToken(user, us.jwt)
	if err != nil {
		return nil, errorc.Internal.New().Internal("failed to generate access token").Wrap(err)
	}

	tokens = append(tokens, models.Token{
		Type:      models.TYPE_ACCESS_TOKEN,
		Token:     accessToken.Token,
		ExpiredIn: accessToken.ExpiredIn,
	})

	refreshToken, err := jwtc.SignRefreshToken(user, us.jwt)
	if err != nil {
		return nil, errorc.Internal.New().Internal("failed to generate refresh token").Wrap(err)
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

// RefreshTokens validates a refresh token, looks up its user, claims its JTI
// (revoking it), and issues a fresh access+refresh pair. The claim is atomic,
// so of several concurrent requests with one token only the first mints a
// pair. It runs after the lookup, so a failed lookup leaves the token usable
// for a retry.
func (us *UserService) RefreshTokens(ctx context.Context, refreshToken string) (*models.GetUserTokenResponse, error) {
	claims, err := jwtc.ParseRefreshToken(refreshToken, us.jwt)
	if err != nil {
		return nil, errorc.Unauthorized.New().Public(herr.Message("invalid refresh token")).Wrap(err)
	}

	user, err := us.store.UserByID(ctx, claims.UserID)
	if errors.Is(err, models.ErrNotFound) {
		return nil, errorc.Unauthorized.New().Public(herr.Message("invalid refresh token")).Wrap(err)
	}
	if err != nil {
		return nil, errorc.Database.New().Internal("failed to get user").Wrap(err)
	}

	claimed, err := us.revoke(ctx, claims, us.jwt.RefreshTokenDuration)
	if err != nil {
		return nil, errorc.Internal.New().Internal("failed to revoke refresh token").Wrap(err)
	}
	if !claimed {
		return nil, errorc.Unauthorized.New().Public(herr.Message("refresh token revoked"))
	}

	return us.issueTokens(user)
}

// Logout ends the caller's session: it revokes the access token so it stops
// being accepted before its natural expiry, and — when a refresh token is
// presented — revokes the refresh token too so it can no longer mint new
// access tokens. An unparseable/expired refresh token is ignored: it cannot
// be used anyway, and logout must stay idempotent.
func (us *UserService) Logout(ctx context.Context, access jwtc.Claims, refreshToken string) error {
	if access.ID != "" {
		if _, err := us.revoke(ctx, &access, us.jwt.AccessTokenDuration); err != nil {
			return errorc.Internal.New().Internal("failed to revoke access token").Wrap(err)
		}
	}

	if refreshToken != "" {
		if claims, err := jwtc.ParseRefreshToken(refreshToken, us.jwt); err == nil {
			if _, err := us.revoke(ctx, claims, us.jwt.RefreshTokenDuration); err != nil {
				return errorc.Internal.New().Internal("failed to revoke refresh token").Wrap(err)
			}
		}
	}

	return nil
}

func (us *UserService) GetByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error) {
	wlog.SetGroup(ctx, "user", map[string]any{
		"operation":      "user.get_by_account_number",
		"account_number": accountNumber,
	})

	user, err := us.store.UserByAccountNumber(ctx, accountNumber)
	if errors.Is(err, models.ErrNotFound) {
		return nil, errorc.DataNotFound.New().Public(herr.Message("User not found")).Wrap(err)
	}
	if err != nil {
		return nil, errorc.Database.New().Internal("failed to get user").Wrap(err)
	}

	return user, nil
}

func (us *UserService) revoke(ctx context.Context, claims *jwtc.Claims, lifetime time.Duration) (first bool, err error) {
	expiresAt := time.Now().Add(lifetime)
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	return us.store.RevokeToken(ctx, claims.ID, expiresAt)
}
