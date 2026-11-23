// Package evaluate は検索精度（recall@k）と出力品質を測る評価ロジックを提供する。
package evaluate

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Code0716/aws-arch-rag/backend/internal/generate"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

// Case は評価ケース1件。Expect に書いた「柱」と「見出しの部分文字列」が、
// 上位 k 件の中に現れてほしいものとして扱う。
type Case struct {
	ID           string `yaml:"id"`
	Requirements string `yaml:"requirements"`
	Expect       Expect `yaml:"expect"`
}

type Expect struct {
	Pillars  []string `yaml:"pillars"`
	Sections []string `yaml:"sections"`
}

// LoadCases は cases.yaml を読み込む。sections が空のケースは、見出し単位の評価ができないので
// 警告（warnings）として返す（エラーにはしない）。
func LoadCases(path string) (cases []Case, warnings []string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if err := yaml.Unmarshal(raw, &cases); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, c := range cases {
		if c.ID == "" || strings.TrimSpace(c.Requirements) == "" {
			return nil, nil, fmt.Errorf("%s: case %d: id と requirements は必須です", path, i)
		}
		if len(c.Expect.Pillars)+len(c.Expect.Sections) == 0 {
			return nil, nil, fmt.Errorf("%s: case %q: expect が空です", path, c.ID)
		}
		if len(c.Expect.Sections) == 0 {
			warnings = append(warnings, fmt.Sprintf("case %q: sections が空です（見出し recall の対象外になります）", c.ID))
		}
	}
	return cases, warnings, nil
}

// Recall は期待項目のうち hits に含まれていた割合。Total が 0 なら期待が無い（評価対象外）。
type Recall struct {
	Value float64
	Found int
	Total int
}

// PillarRecall は期待した柱のうち、hits に現れた割合を返す（完全一致）。
func PillarRecall(e Expect, hits []store.Hit) Recall {
	r := Recall{Total: len(e.Pillars)}
	for _, p := range e.Pillars {
		if slices.ContainsFunc(hits, func(h store.Hit) bool { return h.Pillar == p }) {
			r.Found++
		}
	}
	return r.finish()
}

// SectionRecall は期待した見出しのうち、hits に現れた割合を返す（大文字小文字を無視した部分一致）。
func SectionRecall(e Expect, hits []store.Hit) Recall {
	r := Recall{Total: len(e.Sections)}
	for _, s := range e.Sections {
		if slices.ContainsFunc(hits, func(h store.Hit) bool { return sectionMatches(h, s) }) {
			r.Found++
		}
	}
	return r.finish()
}

func (r Recall) finish() Recall {
	if r.Total > 0 {
		r.Value = float64(r.Found) / float64(r.Total)
	}
	return r
}

func sectionMatches(h store.Hit, expected string) bool {
	return strings.Contains(strings.ToLower(h.Section), strings.ToLower(expected))
}

// FirstSectionRank は、期待した見出しのどれかが最初に現れた順位（1 始まり）を返す。現れなければ 0。
func FirstSectionRank(e Expect, hits []store.Hit) int {
	for i, h := range hits {
		for _, s := range e.Sections {
			if sectionMatches(h, s) {
				return i + 1
			}
		}
	}
	return 0
}

// Score は1ケースの検索結果（hits は順位順、通常は最大の k 件）を、各 k で採点する。
// 見出しの recall を主、柱の recall を参考として別々に計算する。
func Score(c Case, hits []store.Hit, ks []int) CaseResult {
	r := CaseResult{ID: c.ID, Ks: ks, HasSections: len(c.Expect.Sections) > 0}
	for _, k := range ks {
		top := hits[:min(k, len(hits))]
		r.Section = append(r.Section, SectionRecall(c.Expect, top))
		r.Pillar = append(r.Pillar, PillarRecall(c.Expect, top))
	}
	if rank := FirstSectionRank(c.Expect, hits); rank > 0 {
		r.ReciprocalRank = 1 / float64(rank)
	}
	return r
}

var validPillars = map[string]bool{
	"operational-excellence": true, "security": true, "reliability": true,
	"performance-efficiency": true, "cost-optimization": true, "sustainability": true,
}

// CheckSchema は構成案が期待するスキーマに沿っているかを検証し、問題点を返す。
func CheckSchema(p generate.Proposal) []string {
	var problems []string
	if strings.TrimSpace(p.Summary) == "" {
		problems = append(problems, "summary が空")
	}
	if len(p.Components) == 0 {
		problems = append(problems, "components が空")
	}
	if len(p.Rationale) == 0 {
		problems = append(problems, "rationale が空")
	}
	for i, r := range p.Rationale {
		if !validPillars[r.Pillar] {
			problems = append(problems, fmt.Sprintf("rationale[%d].pillar が不正: %q", i, r.Pillar))
		}
		if strings.TrimSpace(r.Text) == "" {
			problems = append(problems, fmt.Sprintf("rationale[%d].text が空", i))
		}
	}
	if len(p.Concerns) == 0 {
		problems = append(problems, "concerns が空")
	}
	if len(p.Questions) == 0 {
		problems = append(problems, "questions が空")
	}
	return problems
}

var mermaidHeader = regexp.MustCompile(`^\s*(flowchart|graph)\s+(TB|TD|BT|RL|LR)\b`)

// CheckMermaid は Mermaid の簡易構文チェック（先頭の宣言・括弧の対応・エッジの存在）。
// 完全な構文検証ではない。厳密に確かめる場合は mermaid-cli（mmdc）かフロントのパーサを使う。
func CheckMermaid(src string) []string {
	var problems []string
	src = strings.TrimSpace(src)
	if !mermaidHeader.MatchString(src) {
		problems = append(problems, "flowchart/graph の宣言で始まっていない")
	}
	if !strings.Contains(src, "-->") && !strings.Contains(src, "---") && !strings.Contains(src, "-.") {
		problems = append(problems, "エッジ(-->)が無い")
	}
	pairs := map[rune]rune{'[': ']', '(': ')', '{': '}'}
	var stack []rune
	inQuote := false
	for _, r := range src {
		switch {
		case r == '"':
			inQuote = !inQuote
		case inQuote:
		case pairs[r] != 0:
			stack = append(stack, pairs[r])
		case r == ']' || r == ')' || r == '}':
			if len(stack) == 0 || stack[len(stack)-1] != r {
				return append(problems, "括弧の対応が取れていない")
			}
			stack = stack[:len(stack)-1]
		}
	}
	if inQuote || len(stack) != 0 {
		problems = append(problems, "括弧または引用符が閉じていない")
	}
	return problems
}
