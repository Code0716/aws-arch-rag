// Package config は環境変数から設定を読む。
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL  string
	VoyageAPIKey string
	EmbedModel   string
	EmbedDim     int
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		VoyageAPIKey: os.Getenv("VOYAGE_API_KEY"),
		EmbedModel:   getenv("EMBED_MODEL", "voyage-3.5"),
	}
	dim, err := strconv.Atoi(getenv("EMBED_DIM", "1024"))
	if err != nil {
		return c, fmt.Errorf("EMBED_DIM: %w", err)
	}
	c.EmbedDim = dim

	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL が未設定です（.env.example を参照）")
	}
	if c.VoyageAPIKey == "" {
		return c, fmt.Errorf("VOYAGE_API_KEY が未設定です（.env.example を参照）")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
