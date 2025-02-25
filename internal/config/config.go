package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	ServerPort   string
	DBConnString string
	RedisURL     string
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func init() {
	logger, _ := zap.NewDevelopment() // Use NewDevelopment for more verbose logging

	// Load configuration
	cfg, err := LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}
	logger.Info("Configuration loaded successfully", zap.String("serverPort", cfg.ServerPort))
}
