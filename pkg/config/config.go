package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// NormalizePort turns a PORT value into the ":8080" form net/http wants,
// accepting either "8080" or ":8080" and falling back to the platform default
// when unset. Cloud Run injects PORT bare, so every service needs this.
func NormalizePort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return ":8080"
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}

func LoadConfig(v *viper.Viper, name string) error {

	v.SetConfigName(name)
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	if err := v.ReadInConfig(); err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if !errors.As(err, &notFoundErr) {
			return fmt.Errorf("read config file(%s) : %w", name, err)
		}
	}

	v.AutomaticEnv()
	return nil
}
