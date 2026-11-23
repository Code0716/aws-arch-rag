// sections は取り込み済みチャンクの見出し（section）を、柱ごとにチャンク数つきで一覧表示する。
// 評価ケース（data/eval/cases.yaml）の sections 期待値を決めるときの参照用。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/Code0716/aws-arch-rag/backend/internal/config"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
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

	rows, err := st.Sections(ctx)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("チャンクがありません（先に make ingest してください）")
	}
	pillar := ""
	for _, r := range rows {
		if r.Pillar != pillar {
			pillar = r.Pillar
			fmt.Printf("\n## %s\n", pillar)
		}
		fmt.Printf("  %3d  %s\n", r.Chunks, r.Section)
	}
	return nil
}
