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

	// API サーバー（Phase 2）。Load では必須にせず、LoadServer で検証する。
	AnthropicAPIKey string
	ClaudeModel     string
	Addr            string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		VoyageAPIKey:    os.Getenv("VOYAGE_API_KEY"),
		EmbedModel:      getenv("EMBED_MODEL", "voyage-4"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeModel:     os.Getenv("CLAUDE_MODEL"),
		Addr:            getenv("ADDR", ":8080"),
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

// LoadServer は Load に加えて、API サーバーに必要な設定（Claude）を検証する。
// モデル名はコードに直接書かず、必ず CLAUDE_MODEL で指定する。
func LoadServer() (Config, error) {
	c, err := Load()
	if err != nil {
		return c, err
	}
	if c.AnthropicAPIKey == "" {
		return c, fmt.Errorf("ANTHROPIC_API_KEY が未設定です（.env.example を参照）")
	}
	if c.ClaudeModel == "" {
		return c, fmt.Errorf("CLAUDE_MODEL が未設定です（例: claude-opus-5-5）")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
