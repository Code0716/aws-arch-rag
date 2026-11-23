package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Code0716/aws-arch-rag/backend/internal/generate"
	"github.com/Code0716/aws-arch-rag/backend/internal/retrieve"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

type fakeRetriever struct{ err error }

func (f fakeRetriever) Search(context.Context, string, int) (retrieve.Result, error) {
	if f.err != nil {
		return retrieve.Result{}, f.err
	}
	return retrieve.Result{
		Hits:        []store.Hit{{ID: 1, Score: 0.9, Pillar: "reliability", Section: "S", Source: "a.txt"}},
		EmbedTokens: 7,
		Duration:    5 * time.Millisecond,
	}, nil
}

type fakeProposer struct{ err error }

func (f fakeProposer) Generate(_ context.Context, _ string, _ []store.Hit, onText func(string)) (generate.Proposal, generate.Usage, error) {
	onText("{")
	if f.err != nil {
		return generate.Proposal{}, generate.Usage{Model: "m"}, f.err
	}
	return generate.Proposal{Summary: "sum", Mermaid: "flowchart LR\nA-->B"},
		generate.Usage{InputTokens: 10, OutputTokens: 5, Model: "m"}, nil
}

type fakeChunks struct{}

func (fakeChunks) Get(_ context.Context, id int64) (store.Chunk, error) {
	if id == 1 {
		return store.Chunk{ID: 1, Source: "a.txt", Pillar: "reliability", Section: "S", Content: "body"}, nil
	}
	return store.Chunk{}, store.ErrNotFound
}

func newServer(ret Retriever, prop Proposer) *httptest.Server {
	s := &Server{Retriever: ret, Proposer: prop, Chunks: fakeChunks{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return httptest.NewServer(s.Handler())
}

func post(t *testing.T, ts *httptest.Server, body string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/propose", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestProposeStreamsEventsInOrder(t *testing.T) {
	ts := newServer(fakeRetriever{}, fakeProposer{})
	defer ts.Close()

	resp, body := post(t, ts, `{"requirements":"ECサイト"}`)
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status=%d ct=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	i1 := strings.Index(body, "event: retrieved")
	i2 := strings.Index(body, "event: delta")
	i3 := strings.Index(body, "event: result")
	if i1 < 0 || i2 < i1 || i3 < i2 {
		t.Errorf("unexpected event order:\n%s", body)
	}
	if !strings.Contains(body, `"chars":1`) || strings.Contains(body, `"text"`) {
		t.Errorf("delta must carry only the char count:\n%s", body)
	}
	if !strings.Contains(body, `"summary":"sum"`) {
		t.Errorf("result missing summary:\n%s", body)
	}
}

func TestProposeEmitsErrorEvent(t *testing.T) {
	ts := newServer(fakeRetriever{}, fakeProposer{err: generate.ErrRefused})
	defer ts.Close()
	_, body := post(t, ts, `{"requirements":"x"}`)
	if !strings.Contains(body, "event: error") || strings.Contains(body, "event: result") {
		t.Errorf("expected only error event:\n%s", body)
	}

	ts2 := newServer(fakeRetriever{err: errors.New("db down")}, fakeProposer{})
	defer ts2.Close()
	_, body = post(t, ts2, `{"requirements":"x"}`)
	if !strings.Contains(body, "event: error") || strings.Contains(body, "db down") {
		t.Errorf("error must be reported without leaking internals:\n%s", body)
	}
}

func TestProposeValidatesInput(t *testing.T) {
	ts := newServer(fakeRetriever{}, fakeProposer{})
	defer ts.Close()
	for name, body := range map[string]string{
		"not json": `nope`,
		"empty":    `{"requirements":""}`,
		"too long": `{"requirements":"` + strings.Repeat("あ", maxRequirementsRunes+1) + `"}`,
	} {
		if resp, _ := post(t, ts, body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
}

func TestGetChunk(t *testing.T) {
	ts := newServer(fakeRetriever{}, fakeProposer{})
	defer ts.Close()
	for path, want := range map[string]int{
		"/api/chunks/1":   200,
		"/api/chunks/999": 404,
		"/api/chunks/abc": 400,
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s: status = %d, want %d", path, resp.StatusCode, want)
		}
	}
}
