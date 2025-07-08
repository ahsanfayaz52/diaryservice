package config

import (
	"os"
)

type Config struct {
	DatabasePath string
	JWTSecret    string
	Port         string
	OpenAIKey    string `json:"openai_key"`
}

func LoadConfig() *Config {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/go-diary.db" // default fallback
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "ahsanapp"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}

	aiKey := os.Getenv("OPENAI")

	return &Config{
		DatabasePath: dbPath,
		JWTSecret:    jwtSecret,
		Port:         port,
		OpenAIKey:    aiKey,
	}
}
