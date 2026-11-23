// search は要件テキストに関連するチャンクの上位 k 件を表示する確認用 CLI。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/Code0716/aws-arch-rag/backend/internal/config"
	"github.com/Code0716/aws-arch-rag/backend/internal/embed"
	"github.com/Code0716/aws-arch-rag/backend/internal/obs"
	"github.com/Code0716/aws-arch-rag/backend/internal/retrieve"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	k := flag.Int("k", 5, "取得件数")
	flag.Parse()
	query := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if query == "" {
		return fmt.Errorf("使い方: search [-k 5] \"要件テキスト\"")
	}

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

	r := &retrieve.Retriever{
		Embed: embed.NewVoyage(cfg.VoyageAPIKey, cfg.EmbedModel, cfg.EmbedDim),
		Store: st,
		Log:   obs.NewLogger(),
	}
	res, err := r.Search(ctx, query, *k)
	if err != nil {
		return err
	}

	for i, h := range res.Hits {
		fmt.Printf("%d. score=%.4f  [%s] %s\n   source=%s  id=%d\n   %s\n\n",
			i+1, h.Score, h.Pillar, h.Section, h.Source, h.ID, preview(h.Content, 200))
	}
	return nil
}

func preview(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
