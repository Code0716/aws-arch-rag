package evaluate

import (
	"slices"
	"testing"

	"github.com/Code0716/aws-arch-rag/backend/internal/generate"
)

// data/eval/cases.yaml が読み込めて、ID の重複や不正な柱名が無いこと。
func TestRepositoryCasesAreValid(t *testing.T) {
	cases, _, err := LoadCases("../../../data/eval/cases.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 10 {
		t.Errorf("cases = %d, want >= 10 (plan.md は 10〜20 件)", len(cases))
	}
	ids := map[string]bool{}
	for _, c := range cases {
		if ids[c.ID] {
			t.Errorf("duplicate id %q", c.ID)
		}
		ids[c.ID] = true
		for _, p := range c.Expect.Pillars {
			if !slices.Contains(generate.Pillars, p) {
				t.Errorf("%s: unknown pillar %q", c.ID, p)
			}
		}
	}
}
