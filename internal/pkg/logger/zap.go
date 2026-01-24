// Package logger provides structured logging initialization with Zap.
//
// This file contains the logger initialization logic that configures Zap
// based on the application environment (development vs production).
//
// Production Configuration:
// - JSON encoding for machine parsing
// - Info level and above
// - Stack traces on errors
// - Caller information included
//
// Development Configuration:
// - Console encoding with colors
// - Debug level and above
// - Stack traces on errors
// - Caller information included
package logger

import (
	"os"

	"go-echo-boilerplate/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log is the raw zap.Logger instance.
// Use logger.Instance (Logger interface) instead for better abstraction.
var Log *zap.Logger

// Initialize configures and initializes the global logger based on environment.
// This should be called once at application startup.
//
// Environment-specific behavior:
//   - Production: JSON encoding, Info level, optimized for log aggregation
//   - Development: Console encoding with colors, Debug level, human-readable
//
// The initialized logger is available via:
//   - logger.Instance (recommended - uses Logger interface)
//   - logger.Log (raw zap.Logger - for advanced use cases)
func Initialize(configuration *config.Configuration) {
	var zapConfig zap.Config
	var encoder zapcore.Encoder

	// Configure based on environment
	isProduction := configuration.Application.Environment == "prod" ||
		configuration.Application.Environment == "production"

	if isProduction {
		// Production: JSON encoding for log aggregation systems
		zapConfig = zap.NewProductionConfig()
		zapConfig.Encoding = "json"
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)

		encoder = zapcore.NewJSONEncoder(zapConfig.EncoderConfig)
	} else {
		// Development: Console encoding with colors for readability
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.Encoding = "console"
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		encoder = zapcore.NewConsoleEncoder(zapConfig.EncoderConfig)
	}

	// Common configuration for all environments
	zapConfig.EncoderConfig.TimeKey = "timestamp"
	zapConfig.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	zapConfig.EncoderConfig.MessageKey = "message"
	zapConfig.EncoderConfig.LevelKey = "level"
	zapConfig.EncoderConfig.CallerKey = "caller"
	zapConfig.EncoderConfig.StacktraceKey = "stacktrace"

	// Create core with configured encoder
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zapConfig.Level,
	)

	// Build logger with options
	Log = zap.New(
		core,
		zap.AddCaller(),                       // Include caller information
		zap.AddStacktrace(zapcore.ErrorLevel), // Stack traces for errors and above
		zap.AddCallerSkip(0),                  // No skip for accurate caller info
	)

	// Replace global zap logger (for libraries using zap.L())
	zap.ReplaceGlobals(Log)

	// Initialize the global Logger interface instance
	Instance = NewZapLogger(Log)

	// Log initialization message
	if isProduction {
		Log.Info("Logger initialized in production mode",
			zap.String("environment", configuration.Application.Environment),
			zap.String("encoding", "json"),
			zap.String("level", "info"),
		)
	} else {
		Log.Info("Logger initialized in development mode",
			zap.String("environment", configuration.Application.Environment),
			zap.String("encoding", "console"),
			zap.String("level", "debug"),
		)
	}
}
