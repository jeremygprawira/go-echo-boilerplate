package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config/configtest"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUsers struct {
	user    *models.User
	barrier *barrier
}

type barrier struct {
	mu      sync.Mutex
	waiting int
	total   int
	open    chan struct{}
}

func (b *barrier) wait() {
	b.mu.Lock()
	b.waiting++
	if b.waiting == b.total {
		close(b.open)
	}
	b.mu.Unlock()
	select {
	case <-b.open:
	case <-time.After(300 * time.Millisecond):
	}
}

func (f *fakeUsers) CreateUser(context.Context, *models.User) error { return nil }
func (f *fakeUsers) UserCredentials(context.Context, string, string) (*models.User, error) {
	return f.user, nil
}
func (f *fakeUsers) UserByAccountNumber(context.Context, string) (*models.User, error) {
	return f.user, nil
}
func (f *fakeUsers) UserByID(context.Context, int) (*models.User, error) {
	if f.barrier != nil {
		f.barrier.wait()
	}
	return f.user, nil
}

type memRevoked struct {
	mu   sync.Mutex
	rows map[string]time.Time
}

func (m *memRevoked) RevokeToken(_ context.Context, jti string, expiresAt time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rows[jti]; ok {
		return false, nil
	}
	m.rows[jti] = expiresAt
	return true, nil
}

func (m *memRevoked) IsTokenRevoked(_ context.Context, jti string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.rows[jti]
	return ok && exp.After(now), nil
}

func (m *memRevoked) PurgeExpiredTokens(context.Context, time.Time) error { return nil }

func revocationTestServer(t *testing.T) http.Handler {
	t.Helper()
	return revocationTestServerWithBarrier(t, nil)
}

func revocationTestServerWithBarrier(t *testing.T, b *barrier) http.Handler {
	t.Helper()
	cfg := configtest.Valid()

	hash, err := generator.Hash("Passw0rd!")
	require.NoError(t, err)
	email := "user@example.test"
	users := &fakeUsers{user: &models.User{ID: 7, AccountNumber: "100", Name: "U", Email: &email, Password: hash}, barrier: b}

	store := struct {
		*fakeUsers
		*memRevoked
	}{users, &memRevoked{rows: map[string]time.Time{}}}

	jwtConfig := jwtc.DefaultConfig(cfg)
	return testServer{
		cfg:        cfg,
		jwt:        jwtConfig,
		revocation: service.NewTokenService(store),
		users:      service.NewUserService(store, jwtConfig),
	}.build(t)
}

func call(t *testing.T, h http.Handler, method, path, bearer string, body any, headers ...string) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", configtest.APIKey)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func tokensOf(t *testing.T, resp map[string]any) (access, refresh string) {
	t.Helper()
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "response has data: %v", resp)
	toks, ok := data["tokens"].([]any)
	require.True(t, ok, "response has tokens: %v", resp)
	require.Len(t, toks, 2)
	return toks[0].(map[string]any)["token"].(string), toks[1].(map[string]any)["token"].(string)
}

func TestRevocation_ServerRunsWithTheRateLimiterOn(t *testing.T) {
	h := revocationTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/tokens", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", configtest.APIKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, "10", rec.Header().Get("X-RateLimit-Limit"))
}

func TestRevocation_LogoutAndRotationInvalidateTokens(t *testing.T) {
	h := revocationTestServer(t)

	code, resp := call(t, h, http.MethodPost, "/api/v1/users/tokens", "",
		map[string]any{"email": "user@example.test", "password": "Passw0rd!"})
	require.Equal(t, http.StatusOK, code, "login: %v", resp)
	_, r1 := tokensOf(t, resp)

	code, resp = call(t, h, http.MethodPost, "/api/v1/users/tokens/refresh", "",
		map[string]any{"refreshToken": r1}, "Idempotency-Key", "retry-1")
	require.Equal(t, http.StatusOK, code, "first refresh: %v", resp)
	a2, r2 := tokensOf(t, resp)

	code, _ = call(t, h, http.MethodPost, "/api/v1/users/tokens/refresh", "",
		map[string]any{"refreshToken": r1}, "Idempotency-Key", "retry-1")
	assert.Equal(t, http.StatusUnauthorized, code, "replaying an already rotated refresh token")

	code, resp = call(t, h, http.MethodPost, "/api/v1/users/logout", a2, map[string]any{"refreshToken": r2})
	require.Equal(t, http.StatusOK, code, "logout: %v", resp)

	code, _ = call(t, h, http.MethodGet, "/api/v1/users/me", a2, nil)
	assert.Equal(t, http.StatusUnauthorized, code, "/me with a logged-out access token")

	code, _ = call(t, h, http.MethodPost, "/api/v1/users/tokens/refresh", "", map[string]any{"refreshToken": r2})
	assert.Equal(t, http.StatusUnauthorized, code, "refresh with a logged-out refresh token")
}

func TestRevocation_ConcurrentRefreshWithOneTokenMintsOnce(t *testing.T) {
	const callers = 5
	h := revocationTestServerWithBarrier(t, &barrier{total: callers, open: make(chan struct{})})

	code, resp := call(t, h, http.MethodPost, "/api/v1/users/tokens", "",
		map[string]any{"email": "user@example.test", "password": "Passw0rd!"})
	require.Equal(t, http.StatusOK, code, "login: %v", resp)
	_, r1 := tokensOf(t, resp)

	codes := make(chan int, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, _ := call(t, h, http.MethodPost, "/api/v1/users/tokens/refresh", "", map[string]any{"refreshToken": r1})
			codes <- c
		}()
	}
	wg.Wait()
	close(codes)

	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	assert.Equal(t, map[int]int{http.StatusOK: 1, http.StatusUnauthorized: callers - 1}, counts)
}
