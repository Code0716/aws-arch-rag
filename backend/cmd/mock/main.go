// mock は API キー・DB なしで、画面の動作確認ができる API サーバー（固定データを返す）。
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Code0716/aws-arch-rag/backend/internal/mock"
	"github.com/Code0716/aws-arch-rag/backend/internal/obs"
	"github.com/Code0716/aws-arch-rag/backend/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "待ち受けアドレス")
	flag.Parse()

	log := obs.NewLogger()
	srv := &server.Server{
		Retriever: mock.Retriever{},
		Proposer:  mock.Proposer{Delay: 40 * time.Millisecond},
		Chunks:    mock.ChunkGetter{},
		Log:       log,
	}
	log.Info("mock server listening (固定データを返します。実際の検索・生成は行いません)", "addr", *addr)
	httpSrv := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	if err := httpSrv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
