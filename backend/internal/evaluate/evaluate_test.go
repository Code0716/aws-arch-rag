package evaluate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Code0716/aws-arch-rag/backend/internal/generate"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

func TestRecallAtK(t *testing.T) {
	hits := []store.Hit{
		{Pillar: "reliability", Section: "Design for failure"},
		{Pillar: "cost-optimization", Section: "Expenditure awareness"},
	}
	e := Expect{Pillars: []string{"reliability", "security"}, Sections: []string{"expenditure"}}
	got, found, total := RecallAtK(e, hits)
	if found != 2 || total != 3 {
		t.Fatalf("found/total = %d/%d, want 2/3", found, total)
	}
	if got < 0.666 || got > 0.667 {
		t.Errorf("recall = %v, want 2/3", got)
	}
	if r, _, _ := RecallAtK(Expect{}, hits); r != 0 {
		t.Errorf("empty expect recall = %v, want 0", r)
	}
}

func TestLoadCases(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	os.WriteFile(good, []byte(`
- id: ec
  requirements: ECサイト
  expect:
    pillars: [reliability]
    sections: ["backup"]
`), 0o600)
	cases, err := LoadCases(good)
	if err != nil || len(cases) != 1 || cases[0].Expect.Pillars[0] != "reliability" {
		t.Fatalf("cases=%+v err=%v", cases, err)
	}

	bad := filepath.Join(dir, "bad.yaml")
	os.WriteFile(bad, []byte("- id: x\n  requirements: y\n"), 0o600)
	if _, err := LoadCases(bad); err == nil {
		t.Error("expected error for case without expect")
	}
}

func TestCheckMermaid(t *testing.T) {
	if p := CheckMermaid("flowchart LR\n  A[\"Web (ALB)\"] --> B[(RDS)]"); len(p) != 0 {
		t.Errorf("valid diagram flagged: %v", p)
	}
	for name, src := range map[string]string{
		"no header":   "A --> B",
		"no edge":     "flowchart LR\n  A[x]",
		"unbalanced":  "flowchart LR\n  A[x --> B",
		"open quote":  "flowchart LR\n  A[\"x] --> B",
		"wrong close": "flowchart LR\n  A[x) --> B",
	} {
		if p := CheckMermaid(src); len(p) == 0 {
			t.Errorf("%s: expected problems", name)
		}
	}
}

func TestCheckSchema(t *testing.T) {
	ok := generate.Proposal{
		Summary:    "s",
		Components: []generate.Component{{Name: "ALB", Role: "r"}},
		Rationale:  []generate.Rationale{{Pillar: "reliability", Text: "t"}},
		Concerns:   []string{"c"},
		Questions:  []string{"q"},
	}
	if p := CheckSchema(ok); len(p) != 0 {
		t.Errorf("valid proposal flagged: %v", p)
	}
	ok.Rationale[0].Pillar = "speed"
	if p := CheckSchema(ok); len(p) != 1 || !strings.Contains(p[0], "pillar") {
		t.Errorf("problems = %v", p)
	}
}

func TestWriteMarkdownSummary(t *testing.T) {
	results := []CaseResult{
		{ID: "a", Recall: 1, Found: 2, Total: 2, Generated: true},
		{ID: "b", Recall: 0.5, Found: 1, Total: 2, Generated: true, DroppedIDs: 1, MermaidProblems: []string{"エッジ(-->)が無い"}},
	}
	s := Summarize(results)
	if s.MeanRecall != 0.75 || s.NoDropped != 1 || s.MermaidOK != 1 {
		t.Errorf("summary = %+v", s)
	}
	var buf bytes.Buffer
	WriteMarkdown(&buf, "baseline", 5, results)
	out := buf.String()
	for _, want := range []string{"baseline (k=5)", "recall@5 (平均) | 0.750", "存在しない引用ID 1 件"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
