// Package server は /api/propose（SSE）と /api/chunks/{id} を提供する。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Code0716/aws-arch-rag/backend/internal/generate"
	"github.com/Code0716/aws-arch-rag/backend/internal/retrieve"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

const (
	maxRequirementsRunes = 4000
	defaultK             = 8
)

type Retriever interface {
	Search(ctx context.Context, query string, k int) (retrieve.Result, error)
}

type Proposer interface {
	Generate(ctx context.Context, requirements string, hits []store.Hit, onText func(string)) (generate.Proposal, generate.Usage, error)
}

type ChunkGetter interface {
	Get(ctx context.Context, id int64) (store.Chunk, error)
}

type Server struct {
	Retriever Retriever
	Proposer  Proposer
	Chunks    ChunkGetter
	Log       *slog.Logger
	K         int
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/propose", s.propose)
	mux.HandleFunc("GET /api/chunks/{id}", s.getChunk)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return mux
}

type proposeRequest struct {
	Requirements string `json:"requirements"`
}

type hitMeta struct {
	ID      int64   `json:"id"`
	Score   float64 `json:"score"`
	Pillar  string  `json:"pillar"`
	Section string  `json:"section"`
	Source  string  `json:"source"`
}

func (s *Server) propose(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := strconv.FormatInt(start.UnixNano(), 36)
	log := s.Log.With("request_id", reqID)

	var in proposeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if in.Requirements == "" {
		http.Error(w, "requirements is required", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(in.Requirements) > maxRequirementsRunes {
		http.Error(w, fmt.Sprintf("requirements must be at most %d characters", maxRequirementsRunes), http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	emit := func(event string, data any) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}

	k := s.K
	if k <= 0 {
		k = defaultK
	}
	ret, err := s.Retriever.Search(r.Context(), in.Requirements, k)
	if err != nil {
		log.Error("propose failed", "stage", "retrieve", "error", err, "total_ms", time.Since(start).Milliseconds())
		emit("error", map[string]string{"message": "検索に失敗しました"})
		return
	}
	metas := make([]hitMeta, len(ret.Hits))
	for i, h := range ret.Hits {
		metas[i] = hitMeta{ID: h.ID, Score: h.Score, Pillar: h.Pillar, Section: h.Section, Source: h.Source}
	}
	emit("retrieved", metas)

	proposal, usage, err := s.Proposer.Generate(r.Context(), in.Requirements, ret.Hits, func(t string) {
		// 断片そのものは JSON の一部で表示に向かないため、進み具合（文字数）だけを流す。
		emit("delta", map[string]int{"chars": utf8.RuneCountInString(t)})
	})
	attrs := []any{
		"model", usage.Model,
		"retrieve_ms", ret.Duration.Milliseconds(),
		"embed_tokens", ret.EmbedTokens,
		"hits", len(ret.Hits),
		"llm_input_tokens", usage.InputTokens,
		"llm_output_tokens", usage.OutputTokens,
		"llm_ms", usage.LLMDuration.Milliseconds(),
		"dropped_chunk_ids", usage.DroppedIDs,
		"total_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		log.Error("propose failed", append(attrs, "stage", "generate", "error", err)...)
		msg := "構成案の生成に失敗しました"
		if errors.Is(err, generate.ErrRefused) {
			msg = "モデルが応答を拒否しました"
		}
		emit("error", map[string]string{"message": msg})
		return
	}
	log.Info("propose done", attrs...)
	emit("result", proposal)
}

func (s *Server) getChunk(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	c, err := s.Chunks.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.Log.Error("get chunk failed", "id", id, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}
