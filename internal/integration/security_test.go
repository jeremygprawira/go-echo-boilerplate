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
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/config/configtest"
	"go-echo-boilerplate/internal/core"

	"github.com/jackc/pgx/v5"
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
		os.Exit(m.Run())
	}
	os.Exit(run(m, dsn))
}

func run(m *testing.M, dsn string) int {
	cfg, conn, err := prepare(dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration:", err)
		return 1
	}
	db = conn
	defer func() { _ = db.Close() }()
	e, _, err := core.Setup(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration: setup:", err)
		return 1
	}
	srv = httptest.NewServer(e)
	defer srv.Close()
	return m.Run()
}

func needDB(t *testing.T) {
	t.Helper()
	if srv == nil {
		t.Skip("TEST_DATABASE_URL is unset. Run make test-integration")
	}
}

func prepare(dsn string) (config.Validated, *sql.DB, error) {
	cfg, err := configFor(dsn)
	if err != nil {
		return config.Validated{}, nil, err
	}
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return config.Validated{}, nil, err
	}
	if err := migrate(conn, "../../migration/db/postgre"); err != nil {
		_ = conn.Close()
		return config.Validated{}, nil, fmt.Errorf("migrate: %w", err)
	}
	return cfg, conn, nil
}

// configFor refuses a DSN that may reach a database whose name does not end
// in _test, because migrate drops the public schema. Query parameters such
// as dbname override the URL path, so it allows none.
func configFor(dsn string) (config.Validated, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return config.Validated{}, err
	}
	if u.RawQuery != "" || strings.Contains(u.Host, ",") {
		return config.Validated{}, fmt.Errorf("TEST_DATABASE_URL must name one host and have no query parameters, because they can change the database")
	}
	pc, err := pgx.ParseConfig(dsn)
	if err != nil {
		return config.Validated{}, err
	}
	if !strings.HasSuffix(pc.Database, "_test") {
		return config.Validated{}, fmt.Errorf("database %q must end in _test, because the suite drops its schema", pc.Database)
	}
	return configtest.Valid(func(c *config.Configuration) {
		c.Application.Timezone = "Asia/Jakarta"
		c.PostgreSQL = config.PostgreSQL{
			Host: pc.Host, Port: int(pc.Port), User: pc.User, Password: pc.Password,
			Name: pc.Database, SSLMode: "disable", MaxOpenConns: 10, MaxIdleConns: 10,
		}
		c.RateLimit.Burst = 50
	}), nil
}

// migrate applies the goose Up section of each migration, oldest first, to
// an empty public schema.
func migrate(db *sql.DB, dir string) error {
	var name string
	if err := db.QueryRow("SELECT current_database()").Scan(&name); err != nil {
		return err
	}
	if !strings.HasSuffix(name, "_test") {
		return fmt.Errorf("connected to database %q, which does not end in _test", name)
	}
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
		CreatedAt time.Time `json:"createdAt"`
	} `json:"data"`
}

func post(t *testing.T, path, bearer string, body any) (int, reply) {
	t.Helper()
	return do(t, http.MethodPost, path, bearer, body)
}

func do(t *testing.T, method, path, bearer string, body any) (int, reply) {
	t.Helper()
	needDB(t)
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

var emails atomic.Int64

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// email returns an address unused in this process, because the schema resets
// once per process and -count or -shuffle rerun tests in it.
func email(t *testing.T) string {
	return fmt.Sprintf("%s-%d@example.test", nonAlnum.ReplaceAllString(strings.ToLower(t.Name()), "-"), emails.Add(1))
}

func register(t *testing.T, email string) reply {
	t.Helper()
	code, out := post(t, "/api/v1/users", "", map[string]any{"name": "Integration", "email": email, "password": password})
	require.Equal(t, http.StatusCreated, code, "register %s: %+v", email, out)
	return out
}

func login(t *testing.T, email string) (access, refresh string) {
	t.Helper()
	code, out := post(t, "/api/v1/users/tokens", "", map[string]any{"email": email, "password": password})
	require.Equal(t, http.StatusOK, code, "login %s: %+v", email, out)
	require.Len(t, out.Data.Tokens, 2, "login returns an access and a refresh token")
	return out.Data.Tokens[0].Token, out.Data.Tokens[1].Token
}

func TestLogin_UnknownAccountAndWrongPasswordLookTheSame(t *testing.T) {
	mail := email(t)
	register(t, mail)

	unknownCode, unknown := post(t, "/api/v1/users/tokens", "", map[string]any{"email": email(t), "password": password})
	wrongCode, wrong := post(t, "/api/v1/users/tokens", "", map[string]any{"email": mail, "password": "WrongPassw0rd!"})

	assert.Equal(t, http.StatusUnauthorized, wrongCode, "wrong password")
	assert.NotEmpty(t, wrong.ErrorCode, "wrong password has an error code")
	assert.Equal(t, wrongCode, unknownCode, "unknown account answers like a wrong password")
	assert.Equal(t, wrong.ErrorCode, unknown.ErrorCode, "unknown account has the wrong-password error code")
}

func TestLogout_RevokesAccessAndRefreshTokens(t *testing.T) {
	mail := email(t)
	register(t, mail)
	access, refresh := login(t, mail)

	code, out := post(t, "/api/v1/users/logout", access, map[string]any{"refreshToken": refresh})
	require.Equal(t, http.StatusOK, code, "logout: %+v", out)

	code, _ = do(t, http.MethodGet, "/api/v1/users/me", access, nil)
	assert.Equal(t, http.StatusUnauthorized, code, "/me with a logged-out access token")
	code, _ = post(t, "/api/v1/users/tokens/refresh", "", map[string]any{"refreshToken": refresh})
	assert.Equal(t, http.StatusUnauthorized, code, "refresh with a logged-out refresh token")
}

func TestRefresh_ConcurrentUseOfOneTokenMintsOnce(t *testing.T) {
	const callers, rounds = 5, 5
	mail := email(t)
	register(t, mail)
	_, refresh := login(t, mail)

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
	mail := email(t)
	register(t, mail)
	_, refresh := login(t, mail)

	_, err := db.Exec("DELETE FROM users WHERE email = $1", mail)
	require.NoError(t, err)

	code, _ := post(t, "/api/v1/users/tokens/refresh", "", map[string]any{"refreshToken": refresh})
	assert.Equal(t, http.StatusUnauthorized, code, "refresh for a deleted user")
}

func TestMe_ReturnsTheCreatedAtThatRegisterReturned(t *testing.T) {
	mail := email(t)
	registered := register(t, mail)
	access, _ := login(t, mail)

	code, me := do(t, http.MethodGet, "/api/v1/users/me", access, nil)
	require.Equal(t, http.StatusOK, code)
	assert.WithinDuration(t, registered.Data.CreatedAt, me.Data.CreatedAt, time.Millisecond,
		"/me returns the instant that register returned, with application.timezone Asia/Jakarta")
}

func TestSoftDeletedUser_CannotLogInReadMeOrRefresh(t *testing.T) {
	mail := email(t)
	register(t, mail)
	access, refresh := login(t, mail)

	_, err := db.Exec("UPDATE users SET deleted_at = now() WHERE email = $1", mail)
	require.NoError(t, err)

	unknownCode, unknown := post(t, "/api/v1/users/tokens", "", map[string]any{"email": email(t), "password": password})
	code, out := post(t, "/api/v1/users/tokens", "", map[string]any{"email": mail, "password": password})
	assert.Equal(t, http.StatusUnauthorized, code, "login for a soft-deleted user")
	assert.Equal(t, unknownCode, code, "a soft-deleted user logs in like an unknown account")
	assert.Equal(t, unknown.ErrorCode, out.ErrorCode, "a soft-deleted user has the unknown-account error code")

	code, _ = do(t, http.MethodGet, "/api/v1/users/me", access, nil)
	assert.Equal(t, http.StatusNotFound, code, "/me with a token issued before the soft delete")
	code, _ = post(t, "/api/v1/users/tokens/refresh", "", map[string]any{"refreshToken": refresh})
	assert.Equal(t, http.StatusUnauthorized, code, "refresh for a soft-deleted user")
}

// TestSetup_RefusesDatabasesNotNamedTest points the setup at a database with
// a marker table through each DSN form that names it, then checks the marker
// survived.
func TestSetup_RefusesDatabasesNotNamedTest(t *testing.T) {
	needDB(t)
	const target = "guard_target"
	_, err := db.Exec("DROP DATABASE IF EXISTS " + target + " WITH (FORCE)")
	require.NoError(t, err)
	_, err = db.Exec("CREATE DATABASE " + target)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec("DROP DATABASE IF EXISTS " + target + " WITH (FORCE)") })

	u, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	require.NoError(t, err)
	at := func(path, query string) string {
		v := *u
		v.Path, v.RawQuery = path, query
		return v.String()
	}
	direct, err := sql.Open("pgx", at("/"+target, ""))
	require.NoError(t, err)
	t.Cleanup(func() { _ = direct.Close() })
	_, err = direct.Exec("CREATE TABLE marker (id int)")
	require.NoError(t, err)

	t.Setenv("PGDATABASE", target)
	for form, dsn := range map[string]string{
		"path":                at("/"+target, ""),
		"dbname param":        at(u.Path, "dbname="+target),
		"database param":      at(u.Path, "database="+target),
		"PGDATABASE, no path": at("", ""),
	} {
		_, conn, err := prepare(dsn)
		if conn != nil {
			_ = conn.Close()
		}
		assert.Error(t, err, form)
	}
	assert.Error(t, migrate(direct, "../../migration/db/postgre"), "migrate on a connection to %s", target)

	var n int
	require.NoError(t, direct.QueryRow("SELECT count(*) FROM marker").Scan(&n), "the marker table survives")
}
