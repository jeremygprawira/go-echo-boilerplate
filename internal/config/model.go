package config

type (
	Configuration struct {
		Application   Application   `mapstructure:"application"`
		PostgreSQL    PostgreSQL    `mapstructure:"postgresql"`
		Authorization Authorization `mapstructure:"authorization"`
		CORS          CORS          `mapstructure:"cors"`
		RateLimit     RateLimit     `mapstructure:"rate_limit"`

		Google   Google   `mapstructure:"google"`
		Redis    Redis    `mapstructure:"redis"`
		Kafka    Kafka    `mapstructure:"kafka"`
		Firebase Firebase `mapstructure:"firebase"`
	}

	Application struct {
		Name        string `mapstructure:"name"`
		Version     string `mapstructure:"version"`
		Port        int    `mapstructure:"port"`
		Environment string `mapstructure:"environment"`
		Host        string `mapstructure:"host"`
		Timeout     int    `mapstructure:"timeout"`
		Timezone    string `mapstructure:"timezone"`
		MaxBodySize string `mapstructure:"max_body_size"`
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
		// being evicted from the in-memory store (Go duration string, e.g. "3m").
		ExpiresIn string `mapstructure:"expires_in"`
	}

	PostgreSQL struct {
		Name            string `mapstructure:"name"`
		User            string `mapstructure:"user"`
		Password        string `mapstructure:"password"`
		Host            string `mapstructure:"host"`
		Port            int    `mapstructure:"port"`
		SSLMode         string `mapstructure:"ssl_mode"`
		MaxIdleConns    int    `mapstructure:"max_idle_conns"`
		MaxOpenConns    int    `mapstructure:"max_open_conns"`
		ConnMaxLifetime string `mapstructure:"conn_max_lifetime"`
	}

	Authorization struct {
		Issuer  string             `mapstructure:"issuer"`
		Access  TokenConfiguration `mapstructure:"access"`
		Refresh TokenConfiguration `mapstructure:"refresh"`
		APIKey  string             `mapstructure:"api_key"`
	}

	TokenConfiguration struct {
		Secret   string `mapstructure:"secret"`
		Duration string `mapstructure:"duration"`
	}

	Google struct {
		ClientID     string `mapstructure:"client_id"`
		ClientSecret string `mapstructure:"client_secret"`
		RedirectURI  string `mapstructure:"redirect_uri"`
		State        string `mapstructure:"state"`
		UserInfoURL  string `mapstructure:"user_info_url"`
	}

	// Redis configures the optional Redis-backed cache/token-revocation client.
	Redis struct {
		Enabled  bool   `mapstructure:"enabled"`
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	}

	// Kafka configures the optional Kafka publisher/consumer clients.
	Kafka struct {
		Enabled bool     `mapstructure:"enabled"`
		Brokers []string `mapstructure:"brokers"`
		GroupID string   `mapstructure:"group_id"`
	}

	// Firebase configures the optional Firebase/Firestore client.
	Firebase struct {
		Enabled         bool   `mapstructure:"enabled"`
		ProjectID       string `mapstructure:"project_id"`
		CredentialsFile string `mapstructure:"credentials_file"`
	}
)
