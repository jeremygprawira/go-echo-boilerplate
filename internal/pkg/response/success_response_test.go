package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/response"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func newContext(t *testing.T, requestID string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	if requestID != "" {
		ctx.Response().Header().Set(echo.HeaderXRequestID, requestID)
	}
	return ctx, rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) models.Response {
	t.Helper()
	var body models.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func requireMetadata(t *testing.T, body models.Response, requestID string) {
	t.Helper()
	require.Equal(t, requestID, body.Metadata.RequestId)
	stamp, err := time.Parse(time.RFC3339, body.Metadata.Timestamp)
	require.NoError(t, err, "timestamp must be RFC3339")
	require.WithinDuration(t, time.Now(), stamp, time.Minute)
}

func TestSuccess(t *testing.T) {
	ctx, rec := newContext(t, "req-1")
	require.NoError(t, response.Success(ctx, http.StatusCreated, map[string]int{"id": 7}))

	require.Equal(t, http.StatusCreated, rec.Code)
	body := decode(t, rec)
	require.Equal(t, http.StatusCreated, body.Code)
	require.Equal(t, "OK", body.Status)
	require.Equal(t, "Request has been successfully processed.", body.Message)
	require.Equal(t, map[string]any{"id": float64(7)}, body.Data)
	requireMetadata(t, body, "req-1")
}

func TestSuccess_FormatsMessage(t *testing.T) {
	ctx, rec := newContext(t, "req-1")
	require.NoError(t, response.Success(ctx, http.StatusOK, nil, "created %s", "alice"))
	require.Equal(t, "created alice", decode(t, rec).Message)
}

func TestSuccess_GeneratesRequestIDWhenUnset(t *testing.T) {
	ctx, rec := newContext(t, "")
	require.NoError(t, response.Success(ctx, http.StatusOK, nil))
	require.NotEmpty(t, decode(t, rec).Metadata.RequestId)
}

func TestSuccessList(t *testing.T) {
	ctx, rec := newContext(t, "req-2")
	require.NoError(t, response.SuccessList(ctx, http.StatusServiceUnavailable, "", []string{"a", "b"}))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := decode(t, rec)
	require.Equal(t, http.StatusServiceUnavailable, body.Code)
	require.Equal(t, "ERROR", body.Status)
	require.Equal(t, 2, body.Metadata.TotalRows)
	requireMetadata(t, body, "req-2")
}

func TestSuccessPagination(t *testing.T) {
	ctx, rec := newContext(t, "req-3")
	page := models.PaginationOutput{NextCursor: "42", HasMore: true, Limit: 20}
	require.NoError(t, response.SuccessPagination(ctx, http.StatusPartialContent, "", page, []int{1, 2}))

	require.Equal(t, http.StatusPartialContent, rec.Code)
	body := decode(t, rec)
	require.Equal(t, http.StatusPartialContent, body.Code)
	require.Equal(t, "OK", body.Status)
	require.Equal(t, &page, body.Pagination)
	require.Equal(t, []any{float64(1), float64(2)}, body.Data)
	requireMetadata(t, body, "req-3")
}
