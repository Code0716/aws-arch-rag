package mock

import (
	"context"
	"testing"

	"github.com/Code0716/aws-arch-rag/backend/internal/evaluate"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

// モックの構成案が、本物の出力に課している検証（スキーマ・Mermaid・引用ID）を満たすこと。
func TestMockProposalPassesTheSameChecks(t *testing.T) {
	if p := evaluate.CheckSchema(Proposal); len(p) != 0 {
		t.Errorf("schema problems: %v", p)
	}
	if p := evaluate.CheckMermaid(Proposal.Mermaid); len(p) != 0 {
		t.Errorf("mermaid problems: %v", p)
	}
	for _, r := range Proposal.Rationale {
		for _, id := range r.ChunkIDs {
			if _, err := (ChunkGetter{}).Get(context.Background(), id); err != nil {
				t.Errorf("rationale cites unknown chunk %d", id)
			}
		}
	}
}

func TestMockChunkGetterNotFound(t *testing.T) {
	if _, err := (ChunkGetter{}).Get(context.Background(), 9999); err != store.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestMockProposerStreamsAllText(t *testing.T) {
	var got string
	_, _, err := Proposer{}.Generate(context.Background(), "x", nil, func(s string) { got += s })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0] != '{' {
		t.Errorf("streamed text = %q", got)
	}
}
