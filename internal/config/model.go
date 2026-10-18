package config

import "time"

type (
	Configuration struct {
		Application   Application   `mapstructure:"application"`
		PostgreSQL    PostgreSQL    `mapstructure:"postgresql"`
		Authorization Authorization `mapstructure:"authorization"`
		CORS          CORS          `mapstructure:"cors"`
		RateLimit     RateLimit     `mapstructure:"rate_limit"`
		Idempotency   Idempotency   `mapstructure:"idempotency"`

		Google Google `mapstructure:"google"`
	}

	Application struct {
		Name        string        `mapstructure:"name"`
		Version     string        `mapstructure:"version"`
		Port        int           `mapstructure:"port"`
		Environment Environment   `mapstructure:"environment"`
		Host        string        `mapstructure:"host"`
		Timeout     time.Duration `mapstructure:"timeout"`
		Timezone    string        `mapstructure:"timezone"`
		MaxBodySize string        `mapstructure:"max_body_size"`
		// TrustedProxies lists the CIDRs of reverse proxies whose
		// X-Forwarded-For entries are believed. Empty means clients connect
		// directly and the TCP peer address identifies them.
		TrustedProxies    []string      `mapstructure:"trusted_proxies"`
		ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
		ReadTimeout       time.Duration `mapstructure:"read_timeout"`
		WriteTimeout      time.Duration `mapstructure:"write_timeout"`
		IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	}

	CORS struct {
		AllowedOrigins []string `mapstructure:"allowed_origins"`
		HeadersAllowed []string `mapstructure:"headers_allowed"`
	}

	// RateLimit configures the auth-endpoint rate limiter (see middleware.RateLimiter).
	// Zero value Enabled would be false, but config.Initialize sets a viper
	// default of true for rate_limit.enabled so an omitted key stays fail-safe
	// (protection on); only an explicit `rate_limit.enabled: false` in YAML
	// turns it off. Rate/Burst/ExpiresIn also get viper defaults (5 req/s,
	// burst 10, 3m) so omitting them keeps sane, non-zero limiter behavior.
	RateLimit struct {
		Enabled bool `mapstructure:"enabled"`
		// Rate is the sustained requests-per-second allowed per client identity.
		Rate float64 `mapstructure:"rate"`
		// Burst is the maximum requests a client can make in a single burst
		// before the sustained Rate applies.
		Burst int `mapstructure:"burst"`
		// ExpiresIn is how long an idle client's bucket is retained before
		// being evicted from the in-memory store.
		ExpiresIn time.Duration `mapstructure:"expires_in"`
	}

	// Idempotency configures how long Idempotency replays a cached response
	// for a repeated Idempotency-Key. config.Initialize defaults ttl to 10m,
	// long enough for a client's own retry window and short enough that a
	// stale entry is not a long-lived surprise.
	Idempotency struct {
		TTL time.Duration `mapstructure:"ttl"`
	}

	PostgreSQL struct {
		Name            string        `mapstructure:"name"`
		User            string        `mapstructure:"user"`
		Password        string        `mapstructure:"password"`
		Host            string        `mapstructure:"host"`
		Port            int           `mapstructure:"port"`
		SSLMode         string        `mapstructure:"ssl_mode"`
		MaxIdleConns    int           `mapstructure:"max_idle_conns"`
		MaxOpenConns    int           `mapstructure:"max_open_conns"`
		ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	}

	Authorization struct {
		Issuer  string             `mapstructure:"issuer"`
		Access  TokenConfiguration `mapstructure:"access"`
		Refresh TokenConfiguration `mapstructure:"refresh"`
		APIKey  string             `mapstructure:"api_key"`
	}

	TokenConfiguration struct {
		Secret   string        `mapstructure:"secret"`
		Duration time.Duration `mapstructure:"duration"`
	}

	Google struct {
		ClientID     string `mapstructure:"client_id"`
		ClientSecret string `mapstructure:"client_secret"`
		RedirectURI  string `mapstructure:"redirect_uri"`
		UserInfoURL  string `mapstructure:"user_info_url"`
	}
)
