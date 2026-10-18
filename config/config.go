package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	AWSWwwRegion string
	SQSQueueURL string
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbUser := os.Getenv("DB_USER")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbName := os.Getenv("DB_NAME")
		if dbHost == "" || dbPort == "" || dbUser == "" || dbPassword == "" || dbName == "" {
			return nil, fmt.Errorf("DATABASE_URL or complete DB_* configuration is required")
		}
		dbURL = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
			url.PathEscape(dbUser), url.PathEscape(dbPassword), dbHost, dbPort, dbName)
	}

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
	}, nil
}
