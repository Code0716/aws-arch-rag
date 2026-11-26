// Package chunk は文書を見出し単位のチャンクに分割する。
package chunk

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Chunk は埋め込み・保存の単位。
type Chunk struct {
	Source  string
	Pillar  string
	Section string
	Content string
	Hash    string
}

// Options は分割パラメータ。トークン数は「4文字 ≒ 1トークン」の近似で扱う。
type Options struct {
	MaxTokens     int
	OverlapTokens int
}

func DefaultOptions() Options {
	return Options{MaxTokens: 512, OverlapTokens: 64}
}

const charsPerToken = 4

// 長さ順ではなく「より具体的なものを先に」判定する。
var pillarKeywords = []struct{ keyword, pillar string }{
	{"operational-excellence", "operational-excellence"},
	{"security", "security"},
	{"reliability", "reliability"},
	{"performance-efficiency", "performance-efficiency"},
	{"cost-optimization", "cost-optimization"},
	{"sustainability", "sustainability"},
	{"serverless", "serverless-lens"},
	{"saas", "saas-lens"},
}

// PillarFromFilename はファイル名から柱（またはレンズ）名を決める。
// 例: reliability-pillar.txt -> reliability
func PillarFromFilename(name string) string {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)))
	base = strings.NewReplacer("_", "-", " ", "-").Replace(base)
	for _, k := range pillarKeywords {
		if strings.Contains(base, k.keyword) {
			return k.pillar
		}
	}
	return "general"
}

// ligatures は pdftotext が出す合字（ﬁ, ﬃ など）を ASCII に戻す。
// そのままだと "Performance eﬃciency" が柱の見出しと一致せず、検索・見出し照合でも "deﬁne" のように崩れる。
var ligatures = strings.NewReplacer(
	"\ufb00", "ff", "\ufb01", "fi", "\ufb02", "fl", "\ufb03", "ffi", "\ufb04", "ffl", "\ufb05", "st", "\ufb06", "st",
)

var pillarHeadings = map[string]string{
	"operational excellence": "operational-excellence",
	"security":               "security",
	"reliability":            "reliability",
	"performance efficiency": "performance-efficiency",
	"cost optimization":      "cost-optimization",
	"sustainability":         "sustainability",
	// 日本語版（docs.aws.amazon.com/ja_jp）の柱の章タイトル
	"運用上の優秀性":   "operational-excellence",
	"セキュリティ":    "security",
	"信頼性":       "reliability",
	"パフォーマンス効率": "performance-efficiency",
	"コスト最適化":    "cost-optimization",
	"持続可能性":     "sustainability",
	// 本編の章タイトルは、概要と表記が異なる
	"コストの最適化":  "cost-optimization",
	"サステナビリティ": "sustainability",
}

// pillarFromHeading は見出しが柱の章タイトル（例: "Reliability" や "Reliability pillar"）なら柱名を返す。
func pillarFromHeading(title string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(title))
	t = strings.TrimSuffix(t, " pillar")
	p, ok := pillarHeadings[t]
	return p, ok
}

// Hash は content の SHA-256（hex）。
func Hash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

var (
	mdHeading       = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
	numberedHeading = regexp.MustCompile(`^\d+(\.\d+)*\.?\s+[A-Z][^.]{2,80}$`)
)

var (
	// bestPracticeHeading は "REL09-BP03 Perform data backup automatically" や "REL09: How do you back up data?" のような、
	// ベストプラクティス・設問の見出し。
	bestPracticeHeading = regexp.MustCompile(`^[A-Z]{2,5} ?\d{1,2}(-BP\d{2})?\b`)
	// numberedStep は "2. Define and automate ...:" のような手順の見出し。
	numberedStep = regexp.MustCompile(`^\d+\.\s`)
)

// genericSubheadings は、どのベストプラクティスにも現れる汎用の小見出し。
// そのままだと "Implementation guidance" が数百チャンクの section になり、見出しから内容を特定できない。
var genericSubheadings = map[string]bool{
	"Implementation guidance":    true,
	"Implementation steps":       true,
	"Resources":                  true,
	"Common issues to watch for": true,
	"Capability intent":          true,
}

// qualifySection は、汎用の小見出しや手順の見出しに、親のベストプラクティスの見出しを前置きする。
// 例: "REL09-BP03 Perform data backup automatically / Implementation guidance"。
// bp は現在のベストプラクティスの見出し（更新して返す）。
func qualifySection(title, bp string) (qualified, newBP string) {
	if _, isPillar := pillarFromHeading(title); isPillar {
		return title, ""
	}
	if bestPracticeHeading.MatchString(title) {
		return title, title
	}
	if bp != "" && (genericSubheadings[title] || numberedStep.MatchString(title)) {
		return bp + " / " + title, bp
	}
	return title, bp
}

func headingOf(line string, prevBlank, nextBlank bool) (string, bool) {
	t := strings.TrimSpace(line)
	if t == "" {
		return "", false
	}
	if m := mdHeading.FindStringSubmatch(t); m != nil {
		return strings.TrimSpace(m[1]), true
	}
	if numberedHeading.MatchString(t) {
		return t, true
	}
	// pdftotext 出力向け: 前後が空行の短い単独行で、文末記号がなく大文字始まり。
	if prevBlank && nextBlank && utf8.RuneCountInString(t) <= 60 && unicode.IsUpper([]rune(t)[0]) &&
		!strings.ContainsAny(t[len(t)-1:], ".,;:") {
		return t, true
	}
	return "", false
}

// Split は text を見出し単位で分け、長いセクションはさらに重なり付きで分割する。
func Split(source, text string, opt Options) []Chunk {
	basePillar := PillarFromFilename(source)
	text = ligatures.Replace(text)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	type section struct {
		title  string
		pillar string
		body   []string
	}
	// 6本の柱が1冊に統合されたフレームワーク文書（ファイル名で柱が決まらない場合）は、
	// 柱の章見出しに出会うたびに現在の柱を切り替える。
	pillar := basePillar
	bp := "" // 現在のベストプラクティスの見出し
	cur := section{title: "(preamble)", pillar: pillar}
	var sections []section
	for i, line := range lines {
		prevBlank := i == 0 || strings.TrimSpace(lines[i-1]) == ""
		nextBlank := i == len(lines)-1 || strings.TrimSpace(lines[i+1]) == ""
		title, ok := headingOf(line, prevBlank, nextBlank)
		if !ok && basePillar == "general" && prevBlank && !strings.HasPrefix(line, " ") {
			// 柱の章見出しは直後が本文で、空行を挟まないことがある（pdftotext 出力）。
			// 行頭が字下げされた表の中などは対象外にする。
			if _, isPillar := pillarFromHeading(line); isPillar {
				title, ok = strings.TrimSpace(line), true
			}
		}
		if ok {
			if basePillar == "general" {
				if p, ok := pillarFromHeading(title); ok {
					pillar = p
				}
			}
			title, bp = qualifySection(title, bp)
			sections = append(sections, cur)
			cur = section{title: title, pillar: pillar}
			continue
		}
		cur.body = append(cur.body, line)
	}
	sections = append(sections, cur)

	var out []Chunk
	for _, s := range sections {
		body := normalize(strings.Join(s.body, "\n"))
		if len(body) < 20 {
			continue
		}
		for _, part := range window(body, opt) {
			out = append(out, Chunk{
				Source:  source,
				Pillar:  s.pillar,
				Section: s.title,
				Content: part,
				Hash:    Hash(part),
			})
		}
	}
	return out
}

// normalize は連続する空白・空行を畳む。
func normalize(s string) string {
	var paras []string
	for _, p := range strings.Split(s, "\n\n") {
		p = strings.Join(strings.Fields(p), " ")
		if p != "" {
			paras = append(paras, p)
		}
	}
	return strings.Join(paras, "\n\n")
}

// window は s を MaxTokens 相当の長さで、OverlapTokens 相当の重なりを持たせて分割する。
func window(s string, opt Options) []string {
	if opt.MaxTokens <= 0 {
		opt = DefaultOptions()
	}
	maxLen := opt.MaxTokens * charsPerToken
	overlap := opt.OverlapTokens * charsPerToken
	if overlap >= maxLen {
		overlap = maxLen / 4
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return []string{s}
	}
	var parts []string
	for start := 0; start < len(r); {
		end := start + maxLen
		if end >= len(r) {
			parts = append(parts, strings.TrimSpace(string(r[start:])))
			break
		}
		// 単語の途中で切らないよう、末尾 20% の範囲で空白まで戻る。
		for cut := end; cut > end-maxLen/5; cut-- {
			if unicode.IsSpace(r[cut]) {
				end = cut
				break
			}
		}
		parts = append(parts, strings.TrimSpace(string(r[start:end])))
		next := end - overlap
		if next <= start {
			next = end
		}
		start = next
	}
	return parts
}
