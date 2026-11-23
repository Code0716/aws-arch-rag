// Package retrieve は要件テキストから関連チャンクを検索する。
package retrieve

import (
	"context"
	"log/slog"
	"time"

	"github.com/Code0716/aws-arch-rag/backend/internal/embed"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

type Searcher interface {
	Search(ctx context.Context, vec []float32, k int) ([]store.Hit, error)
}

type Retriever struct {
	Embed embed.Embedder
	Store Searcher
	Log   *slog.Logger
}

// Result は検索結果と、運用ログ用の計測値。
type Result struct {
	Hits        []store.Hit
	EmbedTokens int
	Duration    time.Duration
}

func (r *Retriever) Search(ctx context.Context, query string, k int) (Result, error) {
	start := time.Now()
	emb, err := r.Embed.Embed(ctx, []string{query}, embed.Query)
	if err != nil {
		return Result{}, err
	}
	hits, err := r.Store.Search(ctx, emb.Vectors[0], k)
	if err != nil {
		return Result{}, err
	}
	res := Result{Hits: hits, EmbedTokens: emb.Tokens, Duration: time.Since(start)}
	r.Log.Info("retrieve",
		"k", k, "hits", len(hits),
		"embed_tokens", res.EmbedTokens,
		"duration_ms", res.Duration.Milliseconds())
	return res, nil
}
