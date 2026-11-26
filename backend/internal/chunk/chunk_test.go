package chunk

import (
	"strings"
	"testing"
)

func TestPillarFromFilename(t *testing.T) {
	cases := map[string]string{
		"reliability-pillar.txt":          "reliability",
		"Cost_Optimization_Pillar.md":     "cost-optimization",
		"data/raw/security-pillar.txt":    "security",
		"serverless-applications-lens.md": "serverless-lens",
		"misc.txt":                        "general",
	}
	for in, want := range cases {
		if got := PillarFromFilename(in); got != want {
			t.Errorf("PillarFromFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitByMarkdownHeading(t *testing.T) {
	text := "# Intro\n\nThis is the introduction section text.\n\n## Design\n\nUse multiple Availability Zones for resilience.\n"
	got := Split("reliability-pillar.md", text, DefaultOptions())
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(got), got)
	}
	if got[0].Section != "Intro" || got[1].Section != "Design" {
		t.Errorf("sections = %q, %q", got[0].Section, got[1].Section)
	}
	for _, c := range got {
		if c.Pillar != "reliability" {
			t.Errorf("pillar = %q", c.Pillar)
		}
		if c.Hash != Hash(c.Content) {
			t.Errorf("hash mismatch for %q", c.Section)
		}
	}
}

func TestSplitSwitchesPillarInFrameworkDocument(t *testing.T) {
	text := "# Reliability\n\n## Foundations\n\nWorkloads must tolerate component failures gracefully.\n\n" +
		"# Cost optimization\n\n## Expenditure awareness\n\nUnderstand where money is being spent on the workload.\n"
	got := Split("wellarchitected-framework.md", text, DefaultOptions())
	if len(got) != 2 {
		t.Fatalf("len = %d: %+v", len(got), got)
	}
	if got[0].Pillar != "reliability" || got[1].Pillar != "cost-optimization" {
		t.Errorf("pillars = %q, %q", got[0].Pillar, got[1].Pillar)
	}
}

func TestSplitNumberedHeadingFromPlainText(t *testing.T) {
	text := "1.2 Foundations\nWorkloads must tolerate component failures gracefully.\n"
	got := Split("reliability.txt", text, DefaultOptions())
	if len(got) != 1 || got[0].Section != "1.2 Foundations" {
		t.Fatalf("got %+v", got)
	}
}

func TestSplitLongSectionHasOverlap(t *testing.T) {
	words := make([]string, 0, 600)
	for i := 0; i < 600; i++ {
		words = append(words, "word"+strings.Repeat("x", i%5))
	}
	text := "# Long\n\n" + strings.Join(words, " ")
	opt := Options{MaxTokens: 100, OverlapTokens: 20} // 400 文字 / 80 文字
	got := Split("cost-optimization.md", text, opt)
	if len(got) < 3 {
		t.Fatalf("expected multiple chunks, got %d", len(got))
	}
	for i, c := range got {
		if len([]rune(c.Content)) > 100*charsPerToken {
			t.Errorf("chunk %d too long: %d", i, len([]rune(c.Content)))
		}
		if i == 0 {
			continue
		}
		// 直前チャンク末尾の語が次チャンク先頭付近に現れる（重なり）
		prevWords := strings.Fields(got[i-1].Content)
		tail := prevWords[len(prevWords)-1]
		if !strings.Contains(c.Content[:min(len(c.Content), 120)], tail) {
			t.Errorf("chunk %d has no overlap with previous (tail %q)", i, tail)
		}
	}
}

func TestSplitIsDeterministic(t *testing.T) {
	text := "# A\n\nSome content that is long enough to keep.\n"
	a := Split("security.md", text, DefaultOptions())
	b := Split("security.md", text, DefaultOptions())
	if len(a) != len(b) || a[0].Hash != b[0].Hash {
		t.Error("Split is not deterministic")
	}
}

// pdftotext の実出力: 柱の章見出しの直後が本文（空行なし）で、"eﬃciency" は合字になる。
func TestSplitDetectsPillarChapterWithoutBlankLineAndLigature(t *testing.T) {
	text := "Intro\n\n" +
		"Reliability\nThe reliability pillar covers the ability of a workload to perform its function correctly.\n\n" +
		"Foundations\n\nBody of foundations that is long enough to be kept as a chunk of its own.\n\n" +
		"Performance e\ufb03ciency\nThe performance e\ufb03ciency pillar covers using resources e\ufb03ciently, and we de\ufb01ne it so.\n"
	got := map[string]string{}
	for _, c := range Split("framework.txt", text, DefaultOptions()) {
		got[c.Section] = c.Pillar
	}
	if got["Reliability"] != "reliability" || got["Foundations"] != "reliability" {
		t.Errorf("reliability chapter not detected: %v", got)
	}
	if got["Performance efficiency"] != "performance-efficiency" {
		t.Errorf("ligature heading not detected: %v", got)
	}
}

// 字下げされた表の中の語（直後が本文）は、柱の章見出しとして扱わない。
func TestSplitIgnoresIndentedPillarWordInTable(t *testing.T) {
	text := "Reliability\nThe reliability pillar covers the ability of a workload to perform its function.\n\n" +
		"                      Security\n                      Protect information, systems, and assets in the table.\n\n" +
		"Foundations\n\nAnother paragraph that is long enough to become its own chunk here.\n"
	for _, c := range Split("framework.txt", text, DefaultOptions()) {
		if c.Pillar != "reliability" {
			t.Errorf("pillar switched by indented text: %+v", c)
		}
	}
}

// 汎用の小見出しの続きのチャンクにも、親のベストプラクティス名が残る。
func TestSplitQualifiesGenericSubheadingsWithBestPractice(t *testing.T) {
	text := "Reliability\nThe reliability pillar covers the ability of a workload to perform its function.\n\n" +
		"REL09-BP03 Perform data backup automatically\n\nIntro paragraph that is long enough to be kept as a chunk.\n\n" +
		"Implementation guidance\n\nUse AWS Backup to automate backups on a schedule for all your data stores.\n\n" +
		"Implementation steps\n\n1. Define the backup plan: Decide the schedule and the retention for each tier.\n\n" +
		"REL10-BP01 Deploy the workload to multiple locations\n\nSecond intro paragraph that is long enough to be kept.\n\n" +
		"Implementation guidance\n\nSpread the workload across several Availability Zones in the Region.\n"
	got := map[string]bool{}
	for _, c := range Split("framework.txt", text, DefaultOptions()) {
		got[c.Section] = true
	}
	for _, want := range []string{
		"REL09-BP03 Perform data backup automatically",
		"REL09-BP03 Perform data backup automatically / Implementation guidance",
		"REL10-BP01 Deploy the workload to multiple locations / Implementation guidance",
	} {
		if !got[want] {
			t.Errorf("section %q missing: %v", want, got)
		}
	}
	// 親が違えば、同じ小見出しでも別の section になる
	if got["Implementation guidance"] {
		t.Errorf("bare generic heading remained: %v", got)
	}
}

// ベストプラクティスの見出しが無い文書（親が分からない）では、汎用の小見出しをそのまま使う。
func TestSplitKeepsGenericSubheadingWithoutParent(t *testing.T) {
	text := "Implementation guidance\n\nSome guidance that is long enough to become its own chunk here.\n"
	cs := Split("notes.txt", text, DefaultOptions())
	if len(cs) != 1 || cs[0].Section != "Implementation guidance" {
		t.Errorf("chunks = %+v", cs)
	}
}
