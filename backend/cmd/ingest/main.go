// ingest は data/raw の文書をチャンク化・埋め込みして pgvector に保存する。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/Code0716/aws-arch-rag/backend/internal/chunk"
	"github.com/Code0716/aws-arch-rag/backend/internal/config"
	"github.com/Code0716/aws-arch-rag/backend/internal/embed"
	"github.com/Code0716/aws-arch-rag/backend/internal/ingest"
	"github.com/Code0716/aws-arch-rag/backend/internal/obs"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	def := chunk.DefaultOptions()
	dir := flag.String("dir", "data/raw", "取り込む文書のディレクトリ（.txt / .md）")
	maxTokens := flag.Int("max-tokens", def.MaxTokens, "チャンクの最大トークン数（目安）")
	overlap := flag.Int("overlap", def.OverlapTokens, "分割時の重なりトークン数（目安）")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	dbDim, err := st.EmbeddingDim(ctx)
	if err != nil {
		return fmt.Errorf("chunks テーブルが見つかりません（make up で migrate してください）: %w", err)
	}
	if dbDim != cfg.EmbedDim {
		return fmt.Errorf("EMBED_DIM=%d と DB の vector(%d) が一致しません。migration を合わせてください", cfg.EmbedDim, dbDim)
	}

	g := &ingest.Ingester{
		Repo:  st,
		Embed: embed.NewVoyage(cfg.VoyageAPIKey, cfg.EmbedModel, cfg.EmbedDim),
		Opt:   chunk.Options{MaxTokens: *maxTokens, OverlapTokens: *overlap},
		Log:   obs.NewLogger(),
	}
	_, err = g.Run(ctx, *dir)
	return err
}
