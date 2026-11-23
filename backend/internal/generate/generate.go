// Package generate は検索で得たチャンクを根拠に、LLM で構成案(JSON)を生成する。
package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

// Proposal は LLM が返す構成案。
type Proposal struct {
	Summary    string      `json:"summary"`
	Components []Component `json:"components"`
	Mermaid    string      `json:"mermaid"`
	Rationale  []Rationale `json:"rationale"`
	Concerns   []string    `json:"concerns"`
	Questions  []string    `json:"questions"`
}

type Component struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type Rationale struct {
	Pillar   string  `json:"pillar"`
	Text     string  `json:"text"`
	ChunkIDs []int64 `json:"chunk_ids"`
}

// LLMResult は LLM 呼び出しの結果と運用ログ用の計測値。
type LLMResult struct {
	Text         string
	InputTokens  int
	OutputTokens int
	Model        string
	StopReason   string
}

// LLM は生成モデルの呼び出し。onText にはストリーミング中のテキスト断片が渡る。
type LLM interface {
	Stream(ctx context.Context, system, user string, onText func(string)) (LLMResult, error)
}

// Usage はリクエスト1件あたりの生成側の計測値。
type Usage struct {
	InputTokens  int
	OutputTokens int
	Model        string
	LLMDuration  time.Duration
	DroppedIDs   []int64
}

type Generator struct {
	LLM LLM
	Log *slog.Logger
}

// ErrRefused はモデルが応答を拒否した場合のエラー。
var ErrRefused = errors.New("model refused the request")

// ErrParse は LLM の出力を構成案として読み取れなかった場合のエラー。
var ErrParse = errors.New("failed to parse LLM output")

// Generate は hits を根拠として構成案を生成し、存在しない chunk_ids を除外して返す。
func (g *Generator) Generate(ctx context.Context, requirements string, hits []store.Hit, onText func(string)) (Proposal, Usage, error) {
	start := time.Now()
	res, err := g.LLM.Stream(ctx, systemPrompt, buildUserPrompt(requirements, hits), onText)
	usage := Usage{
		InputTokens:  res.InputTokens,
		OutputTokens: res.OutputTokens,
		Model:        res.Model,
		LLMDuration:  time.Since(start),
	}
	if err != nil {
		return Proposal{}, usage, err
	}
	if res.StopReason == "refusal" {
		return Proposal{}, usage, ErrRefused
	}
	if res.StopReason == "max_tokens" {
		return Proposal{}, usage, errors.New("LLM output was truncated (max_tokens)")
	}

	p, err := ParseProposal(res.Text)
	if err != nil {
		return Proposal{}, usage, err
	}

	allowed := make(map[int64]bool, len(hits))
	for _, h := range hits {
		allowed[h.ID] = true
	}
	var dropped []int64
	p, dropped = FilterCitations(p, allowed)
	usage.DroppedIDs = dropped
	if len(dropped) > 0 {
		g.Log.Warn("dropped unknown chunk_ids", "chunk_ids", dropped)
	}
	return p, usage, nil
}

// ParseProposal は LLM 出力から JSON を取り出して Proposal にする。
// コードフェンスや前後の説明文が付いていても、最初の { から最後の } までを読む。
func ParseProposal(text string) (Proposal, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return Proposal{}, fmt.Errorf("%w: no JSON object", ErrParse)
	}
	var p Proposal
	dec := json.NewDecoder(strings.NewReader(text[start : end+1]))
	if err := dec.Decode(&p); err != nil {
		return Proposal{}, fmt.Errorf("%w: invalid JSON: %v", ErrParse, err)
	}
	if strings.TrimSpace(p.Summary) == "" || strings.TrimSpace(p.Mermaid) == "" {
		return Proposal{}, fmt.Errorf("%w: missing summary or mermaid", ErrParse)
	}
	return p, nil
}

// FilterCitations は allowed に無い chunk_ids を除外し、除外した ID を返す。
func FilterCitations(p Proposal, allowed map[int64]bool) (Proposal, []int64) {
	var dropped []int64
	out := p
	out.Rationale = make([]Rationale, len(p.Rationale))
	for i, r := range p.Rationale {
		kept := []int64{}
		for _, id := range r.ChunkIDs {
			if allowed[id] {
				kept = append(kept, id)
			} else {
				dropped = append(dropped, id)
			}
		}
		r.ChunkIDs = kept
		out.Rationale[i] = r
	}
	return out, dropped
}
