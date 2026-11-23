package evaluate

import (
	"fmt"
	"io"
	"strings"
)

// CaseResult は1ケースの評価結果。Generated が false の場合、出力評価は行っていない。
type CaseResult struct {
	ID        string
	Recall    float64
	Found     int
	Total     int
	Generated bool
	// 出力評価（Generated のときのみ）
	SchemaProblems  []string
	MermaidProblems []string
	DroppedIDs      int
	GenerateError   string
	// ParseFailed は LLM の出力を構成案として読み取れなかった場合（GenerateError の内数）。
	ParseFailed bool
}

// Summary は全ケースの集計。
type Summary struct {
	Cases        int
	MeanRecall   float64
	Generated    int
	SchemaOK     int
	MermaidOK    int
	NoDropped    int
	GenerateFail int
	ParseFail    int
}

func Summarize(results []CaseResult) Summary {
	s := Summary{Cases: len(results)}
	for _, r := range results {
		s.MeanRecall += r.Recall
		if !r.Generated {
			continue
		}
		s.Generated++
		if r.GenerateError != "" {
			s.GenerateFail++
			if r.ParseFailed {
				s.ParseFail++
			}
			continue
		}
		if len(r.SchemaProblems) == 0 {
			s.SchemaOK++
		}
		if len(r.MermaidProblems) == 0 {
			s.MermaidOK++
		}
		if r.DroppedIDs == 0 {
			s.NoDropped++
		}
	}
	if s.Cases > 0 {
		s.MeanRecall /= float64(s.Cases)
	}
	return s
}

// WriteMarkdown は label / k とともに結果を Markdown の表で出力する。
// 改善の前後で同じ形式の表を並べて比較できる。
func WriteMarkdown(w io.Writer, label string, k int, results []CaseResult) {
	s := Summarize(results)
	fmt.Fprintf(w, "## 評価結果: %s (k=%d)\n\n", label, k)
	fmt.Fprintf(w, "| 指標 | 値 |\n|---|---|\n")
	fmt.Fprintf(w, "| ケース数 | %d |\n", s.Cases)
	fmt.Fprintf(w, "| recall@%d (平均) | %.3f |\n", k, s.MeanRecall)
	if s.Generated > 0 {
		ok := s.Generated - s.GenerateFail
		fmt.Fprintf(w, "| 生成成功 | %d / %d |\n", ok, s.Generated)
		fmt.Fprintf(w, "| JSON パース失敗 | %d / %d |\n", s.ParseFail, s.Generated)
		fmt.Fprintf(w, "| JSON スキーマ適合 | %d / %d |\n", s.SchemaOK, ok)
		fmt.Fprintf(w, "| Mermaid 簡易構文OK | %d / %d |\n", s.MermaidOK, ok)
		fmt.Fprintf(w, "| 引用IDがすべて実在（除外0件） | %d / %d |\n", s.NoDropped, ok)
	}

	fmt.Fprintf(w, "\n| ケース | recall@%d | 見つかった/期待 | 備考 |\n|---|---|---|---|\n", k)
	for _, r := range results {
		fmt.Fprintf(w, "| %s | %.2f | %d/%d | %s |\n", r.ID, r.Recall, r.Found, r.Total, note(r))
	}
}

func note(r CaseResult) string {
	var parts []string
	if r.GenerateError != "" {
		parts = append(parts, "生成失敗: "+r.GenerateError)
	}
	for _, p := range r.SchemaProblems {
		parts = append(parts, "schema: "+p)
	}
	for _, p := range r.MermaidProblems {
		parts = append(parts, "mermaid: "+p)
	}
	if r.DroppedIDs > 0 {
		parts = append(parts, fmt.Sprintf("存在しない引用ID %d 件", r.DroppedIDs))
	}
	return strings.ReplaceAll(strings.Join(parts, "; "), "|", "/")
}
