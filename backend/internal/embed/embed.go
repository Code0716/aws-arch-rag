// Package embed はテキスト埋め込みのプロバイダを Embedder の背後に隠す。
package embed

import "context"

// InputKind は文書側 / クエリ側の区別（Voyage の input_type に対応）。
type InputKind string

const (
	Document InputKind = "document"
	Query    InputKind = "query"
)

// Result は埋め込み結果と、課金対象のトークン数。
type Result struct {
	Vectors [][]float32
	Tokens  int
}

// Embedder は差し替え可能な埋め込みプロバイダ。
type Embedder interface {
	// Embed は texts と同じ順序でベクトルを返す。バッチ分割は実装側の責務。
	Embed(ctx context.Context, texts []string, kind InputKind) (Result, error)
	// Dim は出力ベクトルの次元数。
	Dim() int
}
