package config

import (
	"log"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Database DatabaseConfig `envPrefix:"DATABASE_"`
	Telegram MaxConfig      `envPrefix:"MAX_"`
}

type MaxConfig struct {
	Token string `env:"TOKEN"`
}

type DatabaseConfig struct {
	URI string `env:"URI"`
}

func LoadConfig() (*Config, error) {
	if _, err := os.Stat(".env"); err != nil {
		log.Println(err)
	}

	if err := godotenv.Load(); err != nil {
		log.Println("Error loading .env file:", err)
	}

	var cfg Config
	err := env.Parse(&cfg)

	log.Println(cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
