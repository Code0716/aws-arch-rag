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
	stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: a.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: system}},
		// output_config.format（JSON outputs）で、応答テキストを proposalSchema に適合する JSON に制約する。
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: proposalSchema()},
		},
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
			}
		}
	}
	res := LLMResult{
		Text:         text.String(),
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
