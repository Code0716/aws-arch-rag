package ingest

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Code0716/aws-arch-rag/backend/internal/chunk"
	"github.com/Code0716/aws-arch-rag/backend/internal/embed"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

type fakeEmbedder struct{ calls, texts int }

func (f *fakeEmbedder) Dim() int { return 3 }
func (f *fakeEmbedder) Embed(_ context.Context, texts []string, _ embed.InputKind) (embed.Result, error) {
	f.calls++
	f.texts += len(texts)
	vecs := make([][]float32, len(texts))
	for i := range vecs {
		vecs[i] = []float32{1, 0, 0}
	}
	return embed.Result{Vectors: vecs, Tokens: 10 * len(texts)}, nil
}

type fakeRepo struct{ rows map[string]map[string]bool }

func (r *fakeRepo) Existing(_ context.Context, source string) (map[string]bool, error) {
	if r.rows[source] == nil {
		return map[string]bool{}, nil
	}
	return r.rows[source], nil
}

func (r *fakeRepo) Insert(_ context.Context, rows []store.Row) (int, error) {
	for _, row := range rows {
		if r.rows[row.Source] == nil {
			r.rows[row.Source] = map[string]bool{}
		}
		r.rows[row.Source][store.ExistingKey(row.Section, row.Hash)] = true
	}
	return len(rows), nil
}

func TestRunIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	doc := "# Failure management\n\nDesign workloads to tolerate failure of components.\n\n# Backup\n\nBack up data and test restores regularly.\n"
	if err := os.WriteFile(filepath.Join(dir, "reliability-pillar.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	emb := &fakeEmbedder{}
	g := &Ingester{
		Repo:  &fakeRepo{rows: map[string]map[string]bool{}},
		Embed: emb,
		Opt:   chunk.DefaultOptions(),
		Log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	first, err := g.Run(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.ChunksNew != 2 || first.EmbedTokens != 20 || emb.calls != 1 {
		t.Fatalf("first run = %+v, calls=%d", first, emb.calls)
	}

	second, err := g.Run(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if second.ChunksNew != 0 || second.Skipped != 2 || second.EmbedTokens != 0 {
		t.Fatalf("second run = %+v", second)
	}
	if emb.calls != 1 {
		t.Errorf("embedder called again on re-run: calls=%d", emb.calls)
	}
}

func TestRunErrorsWhenNoFiles(t *testing.T) {
	g := &Ingester{Repo: &fakeRepo{}, Embed: &fakeEmbedder{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if _, err := g.Run(context.Background(), t.TempDir()); err == nil {
		t.Fatal("expected error for empty dir")
	}
}
