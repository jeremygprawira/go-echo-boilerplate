package config

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

func Initialize(ctx context.Context) (Validated, error) {
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
	viper.SetDefault("idempotency.ttl", "10m")
	viper.SetDefault("application.read_header_timeout", "5s")
	viper.SetDefault("application.read_timeout", "15s")
	viper.SetDefault("application.write_timeout", "30s")
	viper.SetDefault("application.idle_timeout", "60s")
	viper.SetDefault("application.timezone", "UTC")

	viper.AddConfigPath("./config")
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return Validated{}, err
	}

	if err := viper.Unmarshal(&configuration, viper.DecodeHook(decodeHook)); err != nil {
		return Validated{}, err
	}

	validated, err := configuration.Validated()
	if err != nil {
		return Validated{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return validated, nil
}

// mapstructure decodes a bare YAML number into a time.Duration as
// nanoseconds, so `read_timeout: 30` would mean 30ns.
var decodeHook = mapstructure.ComposeDecodeHookFunc(
	func(from, to reflect.Type, data any) (any, error) {
		if to == reflect.TypeFor[time.Duration]() && from.Kind() != reflect.String {
			return nil, fmt.Errorf("%v is not a duration, write it with a unit such as \"30s\"", data)
		}
		return data, nil
	},
	mapstructure.TextUnmarshallerHookFunc(),
	mapstructure.StringToTimeDurationHookFunc(),
	mapstructure.StringToSliceHookFunc(","),
)
