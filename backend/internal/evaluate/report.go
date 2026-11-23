package evaluate

import (
	"fmt"
	"io"
	"strings"
)

// CaseResult は1ケースの評価結果。Generated が false の場合、出力評価は行っていない。
// Section / Pillar は Ks と同じ並びで、k ごとの recall を持つ。
type CaseResult struct {
	ID          string
	Ks          []int
	Section     []Recall // 見出し単位（主な指標）
	Pillar      []Recall // 柱単位（参考）
	HasSections bool     // 見出しの期待値があるか。無いケースは見出し recall / MRR の対象外
	// ReciprocalRank は期待した見出しが最初に現れた順位の逆数（現れなければ 0）。
	ReciprocalRank float64
	Generated      bool
	// 出力評価（Generated のときのみ）
	SchemaProblems  []string
	MermaidProblems []string
	DroppedIDs      int
	GenerateError   string
}

// Summary は全ケースの集計。k ごとの平均は Ks と同じ並び。
type Summary struct {
	Cases        int
	SectionCases int // 見出しの期待値があるケース数
	Ks           []int
	MeanSection  []float64 // 見出しの期待値があるケースの平均
	MeanPillar   []float64 // 柱の期待値があるケースの平均
	MRR          float64   // 見出しの期待値があるケースの平均
	Generated    int
	SchemaOK     int
	MermaidOK    int
	NoDropped    int
	GenerateFail int
}

func Summarize(results []CaseResult, ks []int) Summary {
	s := Summary{Cases: len(results), Ks: ks, MeanSection: make([]float64, len(ks)), MeanPillar: make([]float64, len(ks))}
	pillarCases := make([]int, len(ks))
	for _, r := range results {
		if r.HasSections {
			s.SectionCases++
			s.MRR += r.ReciprocalRank
		}
		for i := range ks {
			if i < len(r.Section) && r.Section[i].Total > 0 {
				s.MeanSection[i] += r.Section[i].Value
			}
			if i < len(r.Pillar) && r.Pillar[i].Total > 0 {
				s.MeanPillar[i] += r.Pillar[i].Value
				pillarCases[i]++
			}
		}
		if !r.Generated {
			continue
		}
		s.Generated++
		if r.GenerateError != "" {
			s.GenerateFail++
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
	for i := range ks {
		if s.SectionCases > 0 {
			s.MeanSection[i] /= float64(s.SectionCases)
		}
		if pillarCases[i] > 0 {
			s.MeanPillar[i] /= float64(pillarCases[i])
		}
	}
	if s.SectionCases > 0 {
		s.MRR /= float64(s.SectionCases)
	}
	return s
}

// WriteMarkdown は label / ks とともに結果を Markdown の表で出力する。
// 見出し recall（主な指標）を左、柱 recall（参考）を右に置く。
// 改善の前後で同じ形式の表を並べて比較できる。
func WriteMarkdown(w io.Writer, label string, ks []int, results []CaseResult) {
	s := Summarize(results, ks)
	fmt.Fprintf(w, "## 評価結果: %s (k=%s)\n\n", label, joinInts(ks))
	fmt.Fprintf(w, "| 指標 | 値 |\n|---|---|\n")
	fmt.Fprintf(w, "| ケース数 | %d |\n", s.Cases)
	fmt.Fprintf(w, "| うち見出しの期待値あり | %d |\n", s.SectionCases)
	for i, k := range ks {
		fmt.Fprintf(w, "| 見出し recall@%d (平均) | %s |\n", k, meanCell(s.MeanSection[i], s.SectionCases))
	}
	fmt.Fprintf(w, "| 見出し MRR | %s |\n", meanCell(s.MRR, s.SectionCases))
	for i, k := range ks {
		fmt.Fprintf(w, "| 柱 recall@%d (平均・参考) | %.3f |\n", k, s.MeanPillar[i])
	}
	if s.Generated > 0 {
		ok := s.Generated - s.GenerateFail
		fmt.Fprintf(w, "| 生成成功 | %d / %d |\n", ok, s.Generated)
		fmt.Fprintf(w, "| JSON スキーマ適合 | %d / %d |\n", s.SchemaOK, ok)
		fmt.Fprintf(w, "| Mermaid 簡易構文OK | %d / %d |\n", s.MermaidOK, ok)
		fmt.Fprintf(w, "| 引用IDがすべて実在（除外0件） | %d / %d |\n", s.NoDropped, ok)
	}

	var head, sep strings.Builder
	head.WriteString("| ケース |")
	sep.WriteString("|---|")
	for _, k := range ks {
		fmt.Fprintf(&head, " 見出し@%d |", k)
		sep.WriteString("---|")
	}
	head.WriteString(" MRR |")
	sep.WriteString("---|")
	for _, k := range ks {
		fmt.Fprintf(&head, " 柱@%d |", k)
		sep.WriteString("---|")
	}
	head.WriteString(" 備考 |")
	sep.WriteString("---|")
	fmt.Fprintf(w, "\n%s\n%s\n", head.String(), sep.String())

	for _, r := range results {
		fmt.Fprintf(w, "| %s |", r.ID)
		for i := range ks {
			fmt.Fprintf(w, " %s |", recallCell(r.Section, i))
		}
		if r.HasSections {
			fmt.Fprintf(w, " %.2f |", r.ReciprocalRank)
		} else {
			fmt.Fprint(w, " - |")
		}
		for i := range ks {
			fmt.Fprintf(w, " %s |", recallCell(r.Pillar, i))
		}
		fmt.Fprintf(w, " %s |\n", note(r))
	}
}

func recallCell(rs []Recall, i int) string {
	if i >= len(rs) || rs[i].Total == 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f (%d/%d)", rs[i].Value, rs[i].Found, rs[i].Total)
}

// meanCell は対象ケースが無いとき "-" を返す（0.000 と区別するため）。
func meanCell(v float64, cases int) string {
	if cases == 0 {
		return "-"
	}
	return fmt.Sprintf("%.3f", v)
}

func joinInts(ks []int) string {
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = fmt.Sprint(k)
	}
	return strings.Join(parts, ",")
}

func note(r CaseResult) string {
	var parts []string
	if !r.HasSections {
		parts = append(parts, "sections 未設定")
	}
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
