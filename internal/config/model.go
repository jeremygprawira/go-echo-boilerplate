package config

type (
	Configuration struct {
		Application   Application   `mapstructure:"application"`
		PostgreSQL    PostgreSQL    `mapstructure:"postgresql"`
		Authorization Authorization `mapstructure:"authorization"`
		CORS          CORS          `mapstructure:"cors"`

		Google Google `mapstructure:"google"`
	}

	Application struct {
		Name        string `mapstructure:"name"`
		Version     string `mapstructure:"version"`
		Port        int    `mapstructure:"port"`
		Environment string `mapstructure:"environment"`
		Host        string `mapstructure:"host"`
		Timeout     int    `mapstructure:"timeout"`
		Timezone    string `mapstructure:"timezone"`
	}

	CORS struct {
		HeadersAllowed []string `mapstructure:"headers_allowed"`
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
)
