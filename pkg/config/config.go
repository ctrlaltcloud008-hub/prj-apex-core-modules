package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

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
