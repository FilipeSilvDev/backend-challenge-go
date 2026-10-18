package config

import (
	"os"
)

type Config struct {
	Port 			string
	DatabaseURL 	string
	AWSWwwRegion 	string
	SQSQueueURL 	string
}

func LoadConfig() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/wagering_db?sslmode=disable"
	}

	return &Config{
		Port: 			port,
		DatabaseURL: 	dbURL,
	}, nil
}