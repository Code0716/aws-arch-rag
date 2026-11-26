package retrieve

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Code0716/aws-arch-rag/backend/internal/embed"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

type fakeEmbed struct{ got []string }

func (f *fakeEmbed) Embed(_ context.Context, texts []string, _ embed.InputKind) (embed.Result, error) {
	f.got = append(f.got, texts...)
	return embed.Result{Vectors: [][]float32{{1}}, Tokens: 3}, nil
}
func (f *fakeEmbed) Dim() int { return 1 }

type fakeStore struct{}

func (fakeStore) Search(context.Context, []float32, int) ([]store.Hit, error) {
	return []store.Hit{{ID: 1}}, nil
}

type fakeTranslator struct {
	res TranslateResult
	err error
}

func (f fakeTranslator) Translate(context.Context, string) (TranslateResult, error) {
	return f.res, f.err
}

func newRetriever(e embed.Embedder, t Translator) *Retriever {
	return &Retriever{Embed: e, Store: fakeStore{}, Translate: t, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestSearchWithoutTranslatorEmbedsOriginal(t *testing.T) {
	e := &fakeEmbed{}
	res, err := newRetriever(e, nil).Search(context.Background(), "災害対策", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.got) != 1 || e.got[0] != "災害対策" || res.Query != "災害対策" || res.TranslateTokens != 0 {
		t.Errorf("got embed=%v query=%q translateTokens=%d", e.got, res.Query, res.TranslateTokens)
	}
}

func TestSearchWithTranslatorEmbedsTranslation(t *testing.T) {
	e := &fakeEmbed{}
	tr := fakeTranslator{res: TranslateResult{Text: "disaster recovery", InputTokens: 10, OutputTokens: 4}}
	res, err := newRetriever(e, tr).Search(context.Background(), "災害対策", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.got) != 1 || e.got[0] != "disaster recovery" || res.Query != "disaster recovery" {
		t.Errorf("訳文が埋め込まれていない: embed=%v query=%q", e.got, res.Query)
	}
	if res.TranslateTokens != 14 || len(res.Hits) != 1 {
		t.Errorf("translateTokens=%d hits=%d", res.TranslateTokens, len(res.Hits))
	}
}

func TestSearchReturnsTranslateError(t *testing.T) {
	e := &fakeEmbed{}
	boom := errors.New("boom")
	_, err := newRetriever(e, fakeTranslator{err: boom}).Search(context.Background(), "x", 5)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
	if len(e.got) != 0 {
		t.Errorf("翻訳に失敗したのに埋め込みが呼ばれた: %v", e.got)
	}
}
