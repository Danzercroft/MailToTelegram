package main

import "github.com/spf13/viper"

type Config struct {
	APIToken string `mapstructure:"API_TOKEN"`
	ChatId   int64  `mapstructure:"CHAT_ID"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
