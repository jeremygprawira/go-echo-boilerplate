package logger_test

import (
	"context"
	"errors"
	"testing"

	"go-echo-boilerplate/internal/pkg/logger"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestZapLogger_ErrorFieldLogsTheErrorUnderTheErrorKey(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	logger.NewZapLogger(zap.New(core)).Error(context.Background(), "shutdown failed",
		logger.Error(errors.New("pool closed")), logger.String("process", "cleanup"))

	entries := logs.TakeAll()
	assert.Len(t, entries, 1)
	assert.Equal(t, map[string]any{"error": "pool closed", "process": "cleanup"}, entries[0].ContextMap())
}
