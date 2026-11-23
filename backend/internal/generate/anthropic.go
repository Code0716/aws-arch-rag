package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// Anthropic は Claude API（anthropic-sdk-go）を使う LLM 実装。
type Anthropic struct {
	client    anthropic.Client
	model     string
	maxTokens int64
}

// NewAnthropic は model（環境変数 CLAUDE_MODEL）で指定したモデルを使うクライアントを返す。
func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{
		client:    anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:     model,
		maxTokens: 16000,
	}
}

func (a *Anthropic) Stream(ctx context.Context, system, user string, onText func(string)) (LLMResult, error) {
	props, required := proposalSchema()
	stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: a.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: system}},
		// 現行モデル（claude-opus-5-5 等）は tool_choice の強制指定が 400 になるため auto のまま使う。
		// strict: true でツール入力のスキーマ適合を保証し、呼ばれなかった場合は ParseProposal 側でエラーにする。
		Tools: []anthropic.ToolUnionParam{{OfTool: &anthropic.ToolParam{
			Name:        ToolName,
			Description: anthropic.String(toolDescription),
			Strict:      anthropic.Bool(true),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties:  props,
				Required:    required,
				ExtraFields: map[string]any{"additionalProperties": false},
			},
		}}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})

	var msg anthropic.Message
	var text strings.Builder
	for stream.Next() {
		ev := stream.Current()
		if err := msg.Accumulate(ev); err != nil {
			return LLMResult{}, fmt.Errorf("accumulate stream: %w", err)
		}
		if d, ok := ev.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			switch dd := d.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				text.WriteString(dd.Text)
				if onText != nil {
					onText(dd.Text)
				}
			case anthropic.InputJSONDelta:
				// ツール入力 JSON の断片。進み具合の通知にだけ使う。
				if onText != nil {
					onText(dd.PartialJSON)
				}
			}
		}
	}
	var toolInput []byte
	for _, block := range msg.Content {
		if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok && tu.Name == ToolName {
			toolInput = []byte(tu.Input)
			break
		}
	}
	res := LLMResult{
		Text:         text.String(),
		ToolInput:    toolInput,
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
		Model:        string(msg.Model),
		StopReason:   string(msg.StopReason),
	}
	if err := stream.Err(); err != nil {
		return res, fmt.Errorf("claude stream: %w", err)
	}
	return res, nil
}
