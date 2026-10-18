package logger

import (
	"go-echo-boilerplate/internal/config"

	"github.com/jeremygprawira/wlog"
	wlogherr "github.com/jeremygprawira/wlog/errors/herr"
	"github.com/jeremygprawira/wlog/redact"
)

var sensitiveKeys = []string{
	"password", "passwd", "pwd", "secret", "token", "auth", "authorization", "bearer",
	"api_key", "apikey", "api-key", "access_token", "refresh_token", "id_token",
	"session", "session_id", "sessionid", "cookie",
	"credentials", "credential", "private_key", "privatekey", "public_key", "publickey",
	"cert", "certificate",
	"credit_card", "creditcard", "card_number", "cvv", "cvc", "ssn", "social_security",
	"aws_secret_access_key", "aws_access_key_id", "aws_session_token",
	"db_password", "database_password", "connection_string", "connectionstring",
	"x-api-key", "x-auth-token", "x-access-token", "x-session-id",
}

// NewWlog builds the request wide-event logger. It writes nothing itself: every event
// goes to drain as one entry.
func NewWlog(configuration *config.Configuration, drain wlog.Drain) (*wlog.Logger, error) {
	redactor, err := redact.New(redact.AddKeys(sensitiveKeys...))
	if err != nil {
		return nil, err
	}
	app := configuration.Application
	return wlog.New(
		wlog.WithService(app.Name, app.Version, string(app.Environment)),
		wlog.WithSilent(),
		wlog.WithDrains(drain),
		wlog.WithErrorExtractor(wlogherr.Extractor()),
		// Errors are recorded from the central error handler, so the caller is always
		// the same line and tells a reader nothing.
		wlog.WithCaller(false),
		wlog.WithRedactor(redactor),
	), nil
}
