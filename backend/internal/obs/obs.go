// Package obs は構造化ログ（slog, JSON）の初期化を提供する。
package obs

import (
	"log/slog"
	"os"
)

// NewLogger は JSON 形式で stderr に出力するロガーを返す。
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
