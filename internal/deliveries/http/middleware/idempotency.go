package middleware

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"go-echo-boilerplate/internal/pkg/errorc"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/jeremygprawira/herr"
	"github.com/labstack/echo/v4"
)

const idempotencyKeyHeader = "Idempotency-Key"

// idempotencyMaxEntries bounds the store so a stream of unique keys cannot grow
// memory without limit. A full store stops caching new keys; requests still run.
const idempotencyMaxEntries = 10_000

// idempotencyEntry is a cached response, replayed verbatim for a repeated
// request carrying the same idempotency key.
type idempotencyEntry struct {
	bodyHash    [sha256.Size]byte
	status      int
	body        []byte
	contentType string
	expiresAt   time.Time
}

// idempotencyStore is a minimal in-memory cache keyed by method+path+key. Swap
// for a distributed store (e.g. Redis) to share across instances or survive
// restarts; per the class of operation this guards (auth token rotation),
// an in-memory, single-instance store is an acceptable starting point.
type idempotencyStore struct {
	mu      sync.Mutex
	entries map[string]idempotencyEntry
}

func newIdempotencyStore() *idempotencyStore {
	s := &idempotencyStore{entries: make(map[string]idempotencyEntry)}
	go s.evictLoop()
	return s
}

func (s *idempotencyStore) get(key string) (idempotencyEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return idempotencyEntry{}, false
	}
	return e, true
}

func (s *idempotencyStore) set(key string, e idempotencyEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[key]; !exists && len(s.entries) >= idempotencyMaxEntries {
		return
	}
	s.entries[key] = e
}

func (s *idempotencyStore) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		s.mu.Lock()
		for k, e := range s.entries {
			if now.After(e.expiresAt) {
				delete(s.entries, k)
			}
		}
		s.mu.Unlock()
	}
}

// idempotencyCapture tees everything written through it into buf while still
// writing to the underlying ResponseWriter, so the wrapped handler behaves
// exactly as it would unwrapped while Idempotency also gets a copy to cache.
type idempotencyCapture struct {
	http.ResponseWriter
	buf        bytes.Buffer
	statusCode int
	wrote      bool
}

func (w *idempotencyCapture) WriteHeader(code int) {
	w.statusCode = code
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *idempotencyCapture) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

// Idempotency makes a write endpoint safely retryable: a client that resends
// the same request with the same Idempotency-Key header gets back the exact
// cached response instead of the handler re-executing. It's opt-in — a
// request without the header is unaffected, so callers that don't know about
// idempotency keys keep working exactly as before.
//
// A key is scoped to the caller: the cache key includes a hash of the Cookie
// and Authorization headers, so a replayed response (which can carry freshly
// minted tokens) never reaches a caller who did not present the original
// credential, and one caller cannot see or block another caller's keys. Within
// one caller, reusing a key with a different body is rejected with 422.
//
// Only a successful (2xx) response is cached; a failed attempt can always be
// retried with the same key once whatever caused the failure is fixed.
//
// Known limitation: this does not coalesce concurrent duplicate requests —
// two requests with the same key arriving before the first completes will
// both execute. For the endpoints this is intended for (occasional
// network-retry safety, not high-concurrency dedup), that's an acceptable
// gap; closing it would need a "pending" placeholder and a way for the
// second request to wait or fail fast, which is more machinery than this
// in-memory, single-instance store is meant to provide.
func Idempotency(ttl time.Duration) echo.MiddlewareFunc {
	store := newIdempotencyStore()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.Request().Header.Get(idempotencyKeyHeader)
			if key == "" {
				return next(c)
			}

			caller, bodyHash, err := requestIdentity(c.Request())
			if err != nil {
				return err
			}

			cacheKey := c.Request().Method + " " + c.Path() + " " + caller + " " + key
			if entry, ok := store.get(cacheKey); ok {
				if subtle.ConstantTimeCompare(entry.bodyHash[:], bodyHash[:]) != 1 {
					return errorc.Validation.New().Public(herr.Message("Idempotency-Key was already used with a different request"))
				}
				c.Response().Header().Set("Idempotency-Replayed", "true")
				return c.Blob(entry.status, entry.contentType, entry.body)
			}

			originalWriter := c.Response().Writer
			capture := &idempotencyCapture{ResponseWriter: originalWriter}
			c.Response().Writer = capture

			err = next(c)

			c.Response().Writer = originalWriter

			if err == nil && capture.buf.Len() > 0 && capture.statusCode >= 200 && capture.statusCode < 300 {
				store.set(cacheKey, idempotencyEntry{
					bodyHash:    bodyHash,
					status:      capture.statusCode,
					body:        append([]byte(nil), capture.buf.Bytes()...),
					contentType: capture.Header().Get(echo.HeaderContentType),
					expiresAt:   time.Now().Add(ttl),
				})
			}

			return err
		}
	}
}

// requestIdentity returns a hash of the credentials the caller presented and a
// hash of the request body. The body is restored for the wrapped handler.
func requestIdentity(r *http.Request) (caller string, bodyHash [sha256.Size]byte, err error) {
	if r.Body != nil {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			return "", bodyHash, errorc.InvalidInput.New().Internal("failed to read request body").Wrap(readErr)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		bodyHash = sha256.Sum256(body)
	} else {
		bodyHash = sha256.Sum256(nil)
	}

	h := sha256.New()
	h.Write([]byte(r.Header.Get(echo.HeaderCookie)))
	h.Write([]byte{0})
	h.Write([]byte(r.Header.Get(echo.HeaderAuthorization)))
	return hex.EncodeToString(h.Sum(nil)), bodyHash, nil
}
