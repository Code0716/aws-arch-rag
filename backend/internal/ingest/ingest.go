// Package ingest は文書の取り込み（分割 → 未登録分のみ埋め込み → 保存）を行う。
package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Code0716/aws-arch-rag/backend/internal/chunk"
	"github.com/Code0716/aws-arch-rag/backend/internal/embed"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

// Repository は ingest が必要とする保存先の操作。テストでは fake に差し替える。
type Repository interface {
	Existing(ctx context.Context, source string) (map[string]bool, error)
	Insert(ctx context.Context, rows []store.Row) (int, error)
}

type Ingester struct {
	Repo  Repository
	Embed embed.Embedder
	Opt   chunk.Options
	Log   *slog.Logger
}

// Summary は1回の取り込みの集計。
type Summary struct {
	Files       int
	ChunksTotal int
	ChunksNew   int
	Skipped     int
	EmbedTokens int
	EmbedCalls  int
	Duration    time.Duration
}

// Run は dir 内の .txt / .md をすべて取り込む。
func (g *Ingester) Run(ctx context.Context, dir string) (Summary, error) {
	start := time.Now()
	var sum Summary

	files, err := listFiles(dir)
	if err != nil {
		return sum, err
	}
	if len(files) == 0 {
		return sum, fmt.Errorf("%s に .txt / .md がありません（PDF は make extract で変換）", dir)
	}

	for _, path := range files {
		fs, err := g.ingestFile(ctx, path)
		if err != nil {
			return sum, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		sum.Files++
		sum.ChunksTotal += fs.ChunksTotal
		sum.ChunksNew += fs.ChunksNew
		sum.Skipped += fs.Skipped
		sum.EmbedTokens += fs.EmbedTokens
		sum.EmbedCalls += fs.EmbedCalls
	}
	sum.Duration = time.Since(start)
	g.Log.Info("ingest done",
		"files", sum.Files,
		"chunks_total", sum.ChunksTotal,
		"chunks_new", sum.ChunksNew,
		"chunks_skipped", sum.Skipped,
		"embed_tokens", sum.EmbedTokens,
		"embed_calls", sum.EmbedCalls,
		"duration_ms", sum.Duration.Milliseconds())
	return sum, nil
}

func (g *Ingester) ingestFile(ctx context.Context, path string) (Summary, error) {
	var fs Summary
	raw, err := os.ReadFile(path)
	if err != nil {
		return fs, err
	}
	source := filepath.Base(path)
	chunks := chunk.Split(source, string(raw), g.Opt)
	fs.ChunksTotal = len(chunks)

	existing, err := g.Repo.Existing(ctx, source)
	if err != nil {
		return fs, err
	}
	var pending []chunk.Chunk
	for _, c := range chunks {
		if existing[store.ExistingKey(c.Section, c.Hash)] {
			fs.Skipped++
			continue
		}
		pending = append(pending, c)
	}

	if len(pending) > 0 {
		texts := make([]string, len(pending))
		for i, c := range pending {
			texts[i] = c.Content
		}
		res, err := g.Embed.Embed(ctx, texts, embed.Document)
		if err != nil {
			return fs, err
		}
		if len(res.Vectors) != len(pending) {
			return fs, fmt.Errorf("embedder returned %d vectors for %d chunks", len(res.Vectors), len(pending))
		}
		fs.EmbedTokens = res.Tokens
		fs.EmbedCalls = 1

		rows := make([]store.Row, len(pending))
		for i, c := range pending {
			rows[i] = store.Row{
				Source: c.Source, Pillar: c.Pillar, Section: c.Section,
				Content: c.Content, Hash: c.Hash, Embedding: res.Vectors[i],
			}
		}
		n, err := g.Repo.Insert(ctx, rows)
		if err != nil {
			return fs, err
		}
		fs.ChunksNew = n
	}

	g.Log.Info("ingest file",
		"source", source, "pillar", chunk.PillarFromFilename(source),
		"chunks", fs.ChunksTotal, "new", fs.ChunksNew, "skipped", fs.Skipped,
		"embed_tokens", fs.EmbedTokens)
	return fs, nil
}

func listFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".txt", ".md":
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}
