package logger_test

import (
	"context"
	"testing"

	"go-echo-boilerplate/internal/pkg/logger"

	"go.uber.org/zap"
)

// BenchmarkEnrichContext benchmarks the enrichment operations
func BenchmarkEnrichContext(b *testing.B) {
	// Initialize logger
	zapLog, _ := zap.NewProduction()
	logger.Instance = logger.NewZapLogger(zapLog)

	// Create a wide event
	event := logger.NewWideEvent("req-123", "GET", "/api/test", "127.0.0.1", "test-agent")
	ctx := logger.WithWideEvent(context.Background(), event)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.EnrichContext(ctx, "test_field", "test_value")
	}
}

// BenchmarkEnrichContextMap benchmarks map-based enrichment
func BenchmarkEnrichContextMap(b *testing.B) {
	zapLog, _ := zap.NewProduction()
	logger.Instance = logger.NewZapLogger(zapLog)

	event := logger.NewWideEvent("req-123", "GET", "/api/test", "127.0.0.1", "test-agent")
	ctx := logger.WithWideEvent(context.Background(), event)

	data := map[string]any{
		"field1": "value1",
		"field2": "value2",
		"field3": "value3",
		"field4": "value4",
		"field5": "value5",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.EnrichContextMap(ctx, data)
	}
}

// BenchmarkIsSensitiveField benchmarks the optimized field lookup
func BenchmarkIsSensitiveField(b *testing.B) {
	testFields := []string{
		"username",
		"password",
		"email",
		"api_key",
		"user_password",
		"account_id",
		"session_token",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, field := range testFields {
			// This will use the optimized map-based lookup
			_ = logger.MaskSensitiveData(map[string]any{field: "value"})
		}
	}
}

// BenchmarkMaskSensitiveData benchmarks masking operations
func BenchmarkMaskSensitiveData(b *testing.B) {
	data := map[string]any{
		"username":     "john_doe",
		"password":     "secret123",
		"email":        "john@example.com",
		"api_key":      "sk_live_1234567890",
		"account_id":   "acc_123",
		"session_id":   "sess_abc",
		"phone_number": "+1234567890",
		"address":      "123 Main St",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = logger.MaskSensitiveData(data)
	}
}

// BenchmarkMaskLargeData benchmarks masking with size limit
func BenchmarkMaskLargeData(b *testing.B) {
	// Create a large data structure
	largeData := make(map[string]any, 1000)
	for i := 0; i < 1000; i++ {
		largeData[string(rune(i))] = "some_value_here_that_is_reasonably_long"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Should hit the size limit and return early
		_ = logger.MaskSensitiveData(largeData)
	}
}

// BenchmarkLogWithContext benchmarks logging with context extraction
func BenchmarkLogWithContext(b *testing.B) {
	zapLog, _ := zap.NewProduction()
	log := logger.NewZapLogger(zapLog)

	ctx := context.Background()
	ctx = logger.WithRequestID(ctx, "req-123")
	ctx = logger.WithUserID(ctx, "user-456")
	ctx = logger.WithTraceID(ctx, "trace-789")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info(ctx, "test message",
			logger.String("key1", "value1"),
			logger.Int("key2", 42),
			logger.Bool("key3", true),
		)
	}
}

// BenchmarkBusinessDataSizeLimit benchmarks the size limit enforcement
func BenchmarkBusinessDataSizeLimit(b *testing.B) {
	event := logger.NewWideEvent("req-123", "GET", "/api/test", "127.0.0.1", "test-agent")
	ctx := logger.WithWideEvent(context.Background(), event)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Try to add more than MaxBusinessDataSize entries
		for j := 0; j < 150; j++ {
			logger.EnrichContext(ctx, string(rune(j)), "value")
		}
	}
}

// BenchmarkGetBusinessData benchmarks the optimized GetBusinessData
func BenchmarkGetBusinessData(b *testing.B) {
	event := logger.NewWideEvent("req-123", "GET", "/api/test", "127.0.0.1", "test-agent")

	// Add some data
	for i := 0; i < 50; i++ {
		event.Enrich(string(rune(i)), "value")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.GetBusinessData()
	}
}

// BenchmarkGetBusinessDataEmpty benchmarks GetBusinessData on empty event
func BenchmarkGetBusinessDataEmpty(b *testing.B) {
	event := logger.NewWideEvent("req-123", "GET", "/api/test", "127.0.0.1", "test-agent")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.GetBusinessData()
	}
}
