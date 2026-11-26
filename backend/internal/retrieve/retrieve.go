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

// Translator は検索前にクエリを文書の言語（英語）に訳す。
type Translator interface {
	Translate(ctx context.Context, text string) (TranslateResult, error)
}

// TranslateResult は訳文と、課金対象のトークン数。
type TranslateResult struct {
	Text         string
	InputTokens  int
	OutputTokens int
}

type Retriever struct {
	Embed embed.Embedder
	Store Searcher
	// Translate が nil なら、クエリをそのまま埋め込む。
	Translate Translator
	Log       *slog.Logger
}

// Result は検索結果と、運用ログ用の計測値。
type Result struct {
	Hits []store.Hit
	// Query は実際に埋め込んだ文（翻訳した場合は訳文）。
	Query           string
	EmbedTokens     int
	TranslateTokens int
	Duration        time.Duration
}

func (r *Retriever) Search(ctx context.Context, query string, k int) (Result, error) {
	start := time.Now()
	res := Result{Query: query}
	if r.Translate != nil {
		tr, err := r.Translate.Translate(ctx, query)
		if err != nil {
			return Result{}, err
		}
		res.Query = tr.Text
		res.TranslateTokens = tr.InputTokens + tr.OutputTokens
	}
	emb, err := r.Embed.Embed(ctx, []string{res.Query}, embed.Query)
	if err != nil {
		return Result{}, err
	}
	hits, err := r.Store.Search(ctx, emb.Vectors[0], k)
	if err != nil {
		return Result{}, err
	}
	res.Hits, res.EmbedTokens, res.Duration = hits, emb.Tokens, time.Since(start)
	r.Log.Info("retrieve",
		"k", k, "hits", len(hits),
		"embed_tokens", res.EmbedTokens,
		"translate_tokens", res.TranslateTokens,
		"duration_ms", res.Duration.Milliseconds())
	return res, nil
}
