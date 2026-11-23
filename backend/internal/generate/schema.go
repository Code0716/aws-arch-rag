package generate

// ToolName は構成案を提出させるツールの名前。
const ToolName = "submit_proposal"

// Pillars は Well-Architected の6本の柱。スキーマの enum と検証で共有する。
var Pillars = []string{
	"operational-excellence", "security", "reliability",
	"performance-efficiency", "cost-optimization", "sustainability",
}

const toolDescription = "システム要件に対する AWS 構成案を提出する。回答は必ずこのツールの入力として返す。"

// proposalSchema は Proposal と同じ構造の JSON スキーマ（strict ツール用）。
// strict では全 object に additionalProperties:false と required が必要。
func proposalSchema() (properties map[string]any, required []string) {
	pillars := make([]any, len(Pillars))
	for i, p := range Pillars {
		pillars[i] = p
	}
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	strList := func(desc string) map[string]any {
		return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}}
	}

	properties = map[string]any{
		"summary": str("構成の要約（日本語）"),
		"components": map[string]any{
			"type":        "array",
			"description": "構成要素の一覧",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": str("サービス名（例: ALB）"),
					"role": str("役割（日本語）"),
				},
				"required":             []any{"name", "role"},
				"additionalProperties": false,
			},
		},
		"mermaid": str("構成図の Mermaid 記法。flowchart LR で始める。ノード名は英数字の ID にし、ラベルは [\"...\"] で囲む"),
		"rationale": map[string]any{
			"type":        "array",
			"description": "柱ごとの根拠。<documents> のチャンクに基づいて書く",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pillar": map[string]any{"type": "string", "enum": pillars, "description": "Well-Architected の柱"},
					"text":   str("根拠（日本語）"),
					"chunk_ids": map[string]any{
						"type":        "array",
						"description": "根拠に使った <documents> の chunk_id。存在しない ID は書かない。根拠となるチャンクが無ければ空配列",
						"items":       map[string]any{"type": "integer"},
					},
				},
				"required":             []any{"pillar", "text", "chunk_ids"},
				"additionalProperties": false,
			},
		},
		"concerns":  strList("懸念点（日本語）"),
		"questions": strList("追加で確認したい質問（日本語）"),
	}
	required = []string{"summary", "components", "mermaid", "rationale", "concerns", "questions"}
	return properties, required
}
