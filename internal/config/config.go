package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func Initialize(ctx context.Context) (*Configuration, error) {
	var configuration Configuration

	viper.AutomaticEnv()
	environment := strings.ToLower(viper.GetString("env"))
	configName := fmt.Sprintf("config.%s", environment)

	// Rate limiting on auth endpoints is a security control: default it to
	// enabled so a config file that omits rate_limit entirely doesn't
	// silently disable brute-force protection. Rate/Burst/ExpiresIn get
	// sane non-zero defaults too, so a partially-specified rate_limit block
	// (e.g. only "enabled: true") doesn't zero out the limiter.
	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.rate", 5)
	viper.SetDefault("rate_limit.burst", 10)
	viper.SetDefault("rate_limit.expires_in", "3m")

	viper.AddConfigPath("./config")
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(&configuration); err != nil {
		return nil, err
	}

	if err := configuration.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &configuration, nil
}
