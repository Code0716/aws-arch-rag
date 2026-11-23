package generate

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

type fakeLLM struct {
	res    LLMResult
	err    error
	system string
	user   string
}

func (f *fakeLLM) Stream(_ context.Context, system, user string, onText func(string)) (LLMResult, error) {
	f.system, f.user = system, user
	if onText != nil {
		onText(f.res.Text)
	}
	return f.res, f.err
}

func newGen(llm LLM) *Generator {
	return &Generator{LLM: llm, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

const validJSON = `{"summary":"s","components":[{"name":"ALB","role":"r"}],"mermaid":"flowchart LR\nA-->B",
"rationale":[{"pillar":"reliability","text":"t","chunk_ids":[1,999,2]}],"concerns":["c"],"questions":["q"]}`

func TestGenerateDropsUnknownChunkIDs(t *testing.T) {
	llm := &fakeLLM{res: LLMResult{Text: validJSON, InputTokens: 100, OutputTokens: 50, Model: "m", StopReason: "end_turn"}}
	hits := []store.Hit{{ID: 1, Pillar: "reliability", Section: "A", Content: "x"}, {ID: 2, Content: "y"}}

	p, usage, err := newGen(llm).Generate(context.Background(), "req", hits, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := p.Rationale[0].ChunkIDs
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("chunk_ids = %v, want [1 2]", got)
	}
	if len(usage.DroppedIDs) != 1 || usage.DroppedIDs[0] != 999 {
		t.Errorf("dropped = %v, want [999]", usage.DroppedIDs)
	}
	if usage.InputTokens != 100 || usage.OutputTokens != 50 || usage.Model != "m" {
		t.Errorf("usage = %+v", usage)
	}
}

func TestParseProposalRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"empty":           "",
		"not json":        "no json here",
		"truncated":       `{"summary": `,
		"missing mermaid": `{"summary":"s"}`,
		"bad pillar":      `{"summary":"s","mermaid":"flowchart LR\nA-->B","rationale":[{"pillar":"speed","text":"t","chunk_ids":[]}]}`,
		"empty component": `{"summary":"s","mermaid":"flowchart LR\nA-->B","components":[{"name":"ALB","role":""}]}`,
	}
	for name, in := range cases {
		_, err := ParseProposal([]byte(in))
		if !errors.Is(err, ErrParse) {
			t.Errorf("%s: err = %v, want ErrParse", name, err)
		}
	}
}

func TestParseProposalAcceptsValidInput(t *testing.T) {
	p, err := ParseProposal([]byte(validJSON))
	if err != nil || p.Summary != "s" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}

func TestGenerateFailsOnEmptyOutput(t *testing.T) {
	llm := &fakeLLM{res: LLMResult{Text: "", StopReason: "end_turn"}}
	if _, _, err := newGen(llm).Generate(context.Background(), "r", nil, nil); !errors.Is(err, ErrParse) {
		t.Errorf("err = %v, want ErrParse", err)
	}
}

func TestGenerateRefusalAndTruncation(t *testing.T) {
	_, _, err := newGen(&fakeLLM{res: LLMResult{StopReason: "refusal"}}).Generate(context.Background(), "r", nil, nil)
	if !errors.Is(err, ErrRefused) {
		t.Errorf("err = %v, want ErrRefused", err)
	}
	_, _, err = newGen(&fakeLLM{res: LLMResult{Text: validJSON, StopReason: "max_tokens"}}).Generate(context.Background(), "r", nil, nil)
	if err == nil {
		t.Error("expected error on max_tokens")
	}
}

func TestPromptIncludesChunkIDsAndRequirements(t *testing.T) {
	llm := &fakeLLM{res: LLMResult{Text: validJSON, StopReason: "end_turn"}}
	hits := []store.Hit{{ID: 42, Pillar: "security", Section: `Say "hi"`, Content: "body"}}
	if _, _, err := newGen(llm).Generate(context.Background(), "ECサイト", hits, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`chunk_id="42"`, "body", "ECサイト"} {
		if !strings.Contains(llm.user, want) {
			t.Errorf("user prompt missing %q", want)
		}
	}
	if strings.Contains(llm.user, `section="Say "hi""`) {
		t.Error("section quotes were not escaped")
	}
}
