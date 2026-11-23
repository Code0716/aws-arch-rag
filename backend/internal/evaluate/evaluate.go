// Package evaluate は検索精度（recall@k）と出力品質を測る評価ロジックを提供する。
package evaluate

import (
	"fmt"
	"os"
	"regexp"
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

// LoadCases は cases.yaml を読み込む。
func LoadCases(path string) ([]Case, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []Case
	if err := yaml.Unmarshal(raw, &cases); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, c := range cases {
		if c.ID == "" || strings.TrimSpace(c.Requirements) == "" {
			return nil, fmt.Errorf("%s: case %d: id と requirements は必須です", path, i)
		}
		if len(c.Expect.Pillars)+len(c.Expect.Sections) == 0 {
			return nil, fmt.Errorf("%s: case %q: expect が空です", path, c.ID)
		}
	}
	return cases, nil
}

// RecallAtK は期待項目（柱 + 見出し）のうち、hits に含まれていた割合を返す。
// 柱は完全一致、見出しは大文字小文字を無視した部分一致で判定する。
func RecallAtK(e Expect, hits []store.Hit) (recall float64, found, total int) {
	total = len(e.Pillars) + len(e.Sections)
	if total == 0 {
		return 0, 0, 0
	}
	for _, p := range e.Pillars {
		for _, h := range hits {
			if h.Pillar == p {
				found++
				break
			}
		}
	}
	for _, s := range e.Sections {
		needle := strings.ToLower(s)
		for _, h := range hits {
			if strings.Contains(strings.ToLower(h.Section), needle) {
				found++
				break
			}
		}
	}
	return float64(found) / float64(total), found, total
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
