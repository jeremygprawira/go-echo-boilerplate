package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/config/configtest"
	"go-echo-boilerplate/internal/core"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	srv *httptest.Server
	db  *sql.DB
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "integration: TEST_DATABASE_URL is unset, skipping. Run make test-integration")
		os.Exit(0)
	}
	os.Exit(run(m, dsn))
}

func run(m *testing.M, dsn string) int {
	cfg, err := configFor(dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration:", err)
		return 1
	}
	db, err = sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration:", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	if err := migrate(db, "../../migration/db/postgre"); err != nil {
		fmt.Fprintln(os.Stderr, "integration: migrate:", err)
		return 1
	}
	e, _, err := core.Setup(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration: setup:", err)
		return 1
	}
	srv = httptest.NewServer(e)
	defer srv.Close()
	return m.Run()
}

// configFor refuses a database whose name does not end in _test, because
// migrate drops the public schema.
func configFor(dsn string) (config.Validated, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return config.Validated{}, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return config.Validated{}, fmt.Errorf("port in TEST_DATABASE_URL: %w", err)
	}
	name := strings.TrimPrefix(u.Path, "/")
	if !strings.HasSuffix(name, "_test") {
		return config.Validated{}, fmt.Errorf("database %q must end in _test, because the suite drops its schema", name)
	}
	password, _ := u.User.Password()
	return configtest.Valid(func(c *config.Configuration) {
		c.Application.Timezone = "Asia/Jakarta"
		c.PostgreSQL = config.PostgreSQL{
			Host: u.Hostname(), Port: port, User: u.User.Username(), Password: password,
			Name: name, SSLMode: "disable", MaxOpenConns: 10, MaxIdleConns: 10,
		}
		c.RateLimit.Burst = 50
	}), nil
}

// migrate applies the goose Up section of each migration, oldest first.
func migrate(db *sql.DB, dir string) error {
	if _, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		up, _, _ := strings.Cut(string(raw), "-- +goose Down")
		if _, err := db.Exec(up); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

type reply struct {
	ErrorCode string `json:"errorCode"`
	Data      struct {
		Tokens []struct {
			Token string `json:"token"`
		} `json:"tokens"`
	} `json:"data"`
}

func post(t *testing.T, path, bearer string, body any) (int, reply) {
	t.Helper()
	return do(t, http.MethodPost, path, bearer, body)
}

func do(t *testing.T, method, path, bearer string, body any) (int, reply) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(method, srv.URL+path, bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", configtest.APIKey)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var out reply
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

const password = "Passw0rd!"

func register(t *testing.T, email string) {
	t.Helper()
	code, out := post(t, "/api/v1/users", "", map[string]any{"name": "Integration", "email": email, "password": password})
	require.Equal(t, http.StatusCreated, code, "register %s: %+v", email, out)
}

func login(t *testing.T, email string) (access, refresh string) {
	t.Helper()
	code, out := post(t, "/api/v1/users/tokens", "", map[string]any{"email": email, "password": password})
	require.Equal(t, http.StatusOK, code, "login %s: %+v", email, out)
	require.Len(t, out.Data.Tokens, 2, "login returns an access and a refresh token")
	return out.Data.Tokens[0].Token, out.Data.Tokens[1].Token
}

func TestLogin_UnknownAccountAndWrongPasswordLookTheSame(t *testing.T) {
	register(t, "enum@example.test")

	unknownCode, unknown := post(t, "/api/v1/users/tokens", "", map[string]any{"email": "nobody@example.test", "password": password})
	wrongCode, wrong := post(t, "/api/v1/users/tokens", "", map[string]any{"email": "enum@example.test", "password": "WrongPassw0rd!"})

	assert.Equal(t, http.StatusUnauthorized, wrongCode, "wrong password")
	assert.Equal(t, wrongCode, unknownCode, "unknown account answers like a wrong password")
	assert.Equal(t, wrong.ErrorCode, unknown.ErrorCode, "unknown account has the wrong-password error code")
}

func TestLogout_RevokesAccessAndRefreshTokens(t *testing.T) {
	register(t, "logout@example.test")
	access, refresh := login(t, "logout@example.test")

	code, out := post(t, "/api/v1/users/logout", access, map[string]any{"refreshToken": refresh})
	require.Equal(t, http.StatusOK, code, "logout: %+v", out)

	code, _ = do(t, http.MethodGet, "/api/v1/users/me", access, nil)
	assert.Equal(t, http.StatusUnauthorized, code, "/me with a logged-out access token")
	code, _ = post(t, "/api/v1/users/tokens/refresh", "", map[string]any{"refreshToken": refresh})
	assert.Equal(t, http.StatusUnauthorized, code, "refresh with a logged-out refresh token")
}

func TestRefresh_ConcurrentUseOfOneTokenMintsOnce(t *testing.T) {
	const callers, rounds = 5, 5
	register(t, "race@example.test")
	_, refresh := login(t, "race@example.test")

	for round := range rounds {
		start := make(chan struct{})
		codes := make([]int, callers)
		minted := make([]string, callers)
		var wg sync.WaitGroup
		for i := range callers {
			wg.Go(func() {
				<-start
				var out reply
				codes[i], out = post(t, "/api/v1/users/tokens/refresh", "", map[string]any{"refreshToken": refresh})
				if len(out.Data.Tokens) == 2 {
					minted[i] = out.Data.Tokens[1].Token
				}
			})
		}
		close(start)
		wg.Wait()

		counts := map[int]int{}
		for i, c := range codes {
			counts[c]++
			if minted[i] != "" {
				refresh = minted[i]
			}
		}
		require.Equal(t, map[int]int{http.StatusOK: 1, http.StatusUnauthorized: callers - 1}, counts, "round %d", round+1)
	}
}

func TestRefresh_FailsAfterUserRowIsDeleted(t *testing.T) {
	register(t, "deleted@example.test")
	_, refresh := login(t, "deleted@example.test")

	_, err := db.Exec("DELETE FROM users WHERE email = $1", "deleted@example.test")
	require.NoError(t, err)

	code, _ := post(t, "/api/v1/users/tokens/refresh", "", map[string]any{"refreshToken": refresh})
	assert.Equal(t, http.StatusUnauthorized, code, "refresh for a deleted user")
}
