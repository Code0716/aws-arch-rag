package chunk

import (
	"os"
	"path/filepath"
	"testing"
)

// data/sample のサンプル文書が、想定どおりの柱と見出しでチャンク化されること。
func TestSampleDocsAreChunkedWithPillars(t *testing.T) {
	files, err := filepath.Glob("../../../data/sample/*.md")
	if err != nil || len(files) != 6 {
		t.Fatalf("sample docs: files=%v err=%v", files, err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		chunks := Split(filepath.Base(f), string(raw), DefaultOptions())
		if len(chunks) < 4 {
			t.Errorf("%s: %d chunks, want >= 4", filepath.Base(f), len(chunks))
		}
		for _, c := range chunks {
			if c.Pillar == "general" || c.Section == "(preamble)" {
				t.Errorf("%s: unexpected pillar/section: %q / %q", filepath.Base(f), c.Pillar, c.Section)
			}
			seen[c.Pillar] = true
		}
	}
	if len(seen) != 6 {
		t.Errorf("pillars covered = %v, want 6", seen)
	}
}
