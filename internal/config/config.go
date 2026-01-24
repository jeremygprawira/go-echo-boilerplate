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

	viper.AddConfigPath("./config")
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(&configuration); err != nil {
		return nil, err
	}

	return &configuration, nil
}
