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

func hitsFor(pairs ...string) []store.Hit {
	var hits []store.Hit
	for i := 0; i+1 < len(pairs); i += 2 {
		hits = append(hits, store.Hit{Pillar: pairs[i], Section: pairs[i+1]})
	}
	return hits
}

// 柱は当たっているが見出しは外れている、という柱 recall の上限張り付きを再現する。
func TestPillarAndSectionRecallAreSeparate(t *testing.T) {
	hits := hitsFor("reliability", "Design for failure", "cost-optimization", "Expenditure awareness")
	e := Expect{Pillars: []string{"reliability", "security"}, Sections: []string{"expenditure", "backup"}}

	p := PillarRecall(e, hits)
	if p.Found != 1 || p.Total != 2 || p.Value != 0.5 {
		t.Errorf("pillar = %+v, want 1/2", p)
	}
	s := SectionRecall(e, hits)
	if s.Found != 1 || s.Total != 2 || s.Value != 0.5 {
		t.Errorf("section = %+v, want 1/2", s)
	}

	// 柱だけ期待 → 見出しは評価対象外（Total 0）
	if r := SectionRecall(Expect{Pillars: []string{"reliability"}}, hits); r.Total != 0 {
		t.Errorf("section recall without sections = %+v", r)
	}
	// 柱は全部当たるが見出しは 0
	e2 := Expect{Pillars: []string{"reliability"}, Sections: []string{"backup"}}
	if PillarRecall(e2, hits).Value != 1 || SectionRecall(e2, hits).Value != 0 {
		t.Error("pillar should hit while section misses")
	}
}

func TestSectionMatchIgnoresCase(t *testing.T) {
	hits := hitsFor("reliability", "Back Up Data")
	if r := SectionRecall(Expect{Sections: []string{"back up"}}, hits); r.Found != 1 {
		t.Errorf("recall = %+v", r)
	}
}

func TestScoreCutsHitsPerK(t *testing.T) {
	hits := hitsFor("security", "A", "reliability", "B", "reliability", "Backup and restore", "cost-optimization", "D")
	c := Case{ID: "x", Expect: Expect{Pillars: []string{"cost-optimization"}, Sections: []string{"backup"}}}
	r := Score(c, hits, []int{2, 3, 10})
	wantSection := []float64{0, 1, 1}
	wantPillar := []float64{0, 0, 1}
	for i := range wantSection {
		if r.Section[i].Value != wantSection[i] || r.Pillar[i].Value != wantPillar[i] {
			t.Errorf("k=%d: section=%v pillar=%v", r.Ks[i], r.Section[i].Value, r.Pillar[i].Value)
		}
	}
	// 期待した見出しが最初に現れたのは3位 → 逆順位 1/3
	if r.ReciprocalRank < 0.333 || r.ReciprocalRank > 0.334 {
		t.Errorf("RR = %v, want 1/3", r.ReciprocalRank)
	}
	if !r.HasSections {
		t.Error("HasSections = false")
	}
	// 見つからなければ 0
	c.Expect.Sections = []string{"nowhere"}
	if r := Score(c, hits, []int{5}); r.ReciprocalRank != 0 {
		t.Errorf("RR = %v, want 0", r.ReciprocalRank)
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
- id: no-sections
  requirements: バッチ
  expect:
    pillars: [security]
`), 0o600)
	cases, warnings, err := LoadCases(good)
	if err != nil || len(cases) != 2 || cases[0].Expect.Pillars[0] != "reliability" {
		t.Fatalf("cases=%+v err=%v", cases, err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "no-sections") {
		t.Errorf("warnings = %v, want one for no-sections", warnings)
	}

	bad := filepath.Join(dir, "bad.yaml")
	os.WriteFile(bad, []byte("- id: x\n  requirements: y\n"), 0o600)
	if _, _, err := LoadCases(bad); err == nil {
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
	ks := []int{3, 5}
	results := []CaseResult{
		{ID: "a", Ks: ks, HasSections: true, ReciprocalRank: 1, Generated: true,
			Section: []Recall{{Value: 1, Found: 2, Total: 2}, {Value: 1, Found: 2, Total: 2}},
			Pillar:  []Recall{{Value: 1, Found: 1, Total: 1}, {Value: 1, Found: 1, Total: 1}}},
		{ID: "b", Ks: ks, HasSections: true, ReciprocalRank: 0.5, Generated: true, DroppedIDs: 1,
			MermaidProblems: []string{"エッジ(-->)が無い"},
			Section:         []Recall{{Value: 0, Found: 0, Total: 2}, {Value: 0.5, Found: 1, Total: 2}},
			Pillar:          []Recall{{Value: 1, Found: 1, Total: 1}, {Value: 1, Found: 1, Total: 1}}},
		{ID: "c", Ks: ks, Pillar: []Recall{{Value: 1, Found: 1, Total: 1}, {Value: 1, Found: 1, Total: 1}},
			Section: []Recall{{}, {}}},
	}
	s := Summarize(results, ks)
	if s.SectionCases != 2 || s.MeanSection[0] != 0.5 || s.MeanSection[1] != 0.75 || s.MRR != 0.75 || s.MeanPillar[0] != 1 {
		t.Errorf("summary = %+v", s)
	}
	if s.NoDropped != 1 || s.MermaidOK != 1 {
		t.Errorf("generate summary = %+v", s)
	}
	var buf bytes.Buffer
	WriteMarkdown(&buf, "baseline", ks, results)
	out := buf.String()
	for _, want := range []string{
		"baseline (k=3,5)", "見出し recall@5 (平均) | 0.750", "柱 recall@3 (平均・参考) | 1.000",
		"| ケース | 見出し@3 | 見出し@5 | MRR | 柱@3 | 柱@5 | 備考 |", "存在しない引用ID 1 件", "sections 未設定",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// 見出しの期待値が無いケースは、見出し列が "-"
	if !strings.Contains(out, "| c | - | - | - |") {
		t.Errorf("case without sections should show '-':\n%s", out)
	}
}
