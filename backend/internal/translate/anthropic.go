// Package translate は検索前に要件テキストを英語に訳す（文書が英語のため）。
package translate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/Code0716/aws-arch-rag/backend/internal/retrieve"
)

const systemPrompt = `You translate system requirements for an AWS architecture into English.
Keep AWS service names, numbers and units (for example "RPO 15 minutes"), and technical terms accurate.
Do not add, drop, or interpret any requirement. The text to translate is inside <requirements> tags.
Output only the English translation, with no preamble, no comments and no tags.`

// Anthropic は Claude API で翻訳する retrieve.Translator の実装。
type Anthropic struct {
	client anthropic.Client
	model  string
}

// NewAnthropic は model（環境変数 TRANSLATE_MODEL）で指定したモデルで訳すクライアントを返す。
func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{client: anthropic.NewClient(option.WithAPIKey(apiKey)), model: model}
}

func (a *Anthropic) Translate(ctx context.Context, text string) (retrieve.TranslateResult, error) {
	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 2000,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("<requirements>\n" + text + "\n</requirements>")),
		},
	})
	if err != nil {
		return retrieve.TranslateResult{}, fmt.Errorf("translate: %w", err)
	}
	var out strings.Builder
	for _, b := range msg.Content {
		if b.Type == "text" {
			out.WriteString(b.Text)
		}
	}
	res := retrieve.TranslateResult{
		Text:         strings.TrimSpace(out.String()),
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}
	if res.Text == "" {
		return res, errors.New("translate: empty response (stop_reason=" + string(msg.StopReason) + ")")
	}
	return res, nil
}
