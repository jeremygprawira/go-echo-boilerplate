// Package logger owns the zap backend. Dev uses colored console encoding at debug level,
// prod uses JSON at info level. Request wide events are built by wlog and written
// through the same zap core.
package logger

import (
	"os"

	"go-echo-boilerplate/internal/config"

	"github.com/jeremygprawira/wlog"
	wlogzap "github.com/jeremygprawira/wlog/log/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Initialize builds the zap backend, sets Instance for plain startup and shutdown lines,
// and returns the wlog logger for request events, also set as wlog's default.
func Initialize(configuration *config.Configuration) (*wlog.Logger, error) {
	isProduction := configuration.IsProduction()

	var zapConfig zap.Config
	if isProduction {
		zapConfig = zap.NewProductionConfig()
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	zapConfig.EncoderConfig.TimeKey = "timestamp"
	zapConfig.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	zapConfig.EncoderConfig.MessageKey = "message"
	zapConfig.EncoderConfig.LevelKey = "level"
	zapConfig.EncoderConfig.CallerKey = "caller"
	zapConfig.EncoderConfig.StacktraceKey = "stacktrace"

	newCore := func(encoderConfig zapcore.EncoderConfig) zapcore.Core {
		encoder := zapcore.NewConsoleEncoder(encoderConfig)
		if isProduction {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		}
		return zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zapConfig.Level)
	}

	base := zap.New(newCore(zapConfig.EncoderConfig), zap.AddCaller())
	zap.ReplaceGlobals(base)
	Instance = NewZapLogger(base.WithOptions(zap.AddCallerSkip(1)))

	// A wide event carries its own timestamp, so the drain core omits zap's to avoid a
	// duplicate key. A caller would always point at the drain, so it is left off too.
	eventConfig := zapConfig.EncoderConfig
	eventConfig.TimeKey = zapcore.OmitKey
	log, err := NewWlog(configuration, wlogzap.Drain(zap.New(newCore(eventConfig))))
	if err != nil {
		return nil, err
	}
	wlog.SetDefault(log)

	encoding := "console"
	if isProduction {
		encoding = "json"
	}
	base.Info("Logger initialized",
		zap.String("environment", configuration.Application.Environment),
		zap.String("encoding", encoding),
		zap.String("log_level", zapConfig.Level.String()),
	)
	return log, nil
}
