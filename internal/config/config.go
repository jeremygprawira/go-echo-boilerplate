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
	bindEnvOverrides()
	environment := strings.ToLower(viper.GetString("env"))
	configName := fmt.Sprintf("config.%s", environment)

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

// bindEnvOverrides makes nested config keys overridable by environment variables,
// mapping dotted keys onto underscore-delimited env names
// (e.g. postgresql.password -> POSTGRESQL_PASSWORD). Env values take precedence
// over YAML so deployments can inject secrets without writing files.
func bindEnvOverrides() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}
