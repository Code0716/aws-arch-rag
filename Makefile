-include .env
export

K ?= 5

.PHONY: up down demo demo-ingest demo-ingest-sample mock extract ingest search sections server web evaluate test vet

# PDF なしで試す: data/sample の短いサンプル文書（自作）を取り込む。Voyage の API キーは必要
demo-ingest-sample:
	docker compose run --rm --build ingest -dir /data/sample

# API キー・DB なしで画面を確認する（固定データを返すモック API、:8080）。別ターミナルで make web
mock:
	cd backend && go run ./cmd/mock

# DB + API + フロントを docker compose で一括起動（http://localhost:3000）
demo:
	docker compose up -d --build

# data/raw の文書を compose 上で取り込む（make demo の後に実行）
demo-ingest:
	docker compose run --rm --build ingest

up:
	docker compose up -d --wait db
	docker compose run --rm migrate

down:
	docker compose down

# data/raw/*.pdfをpdftotextでテキスト化する（brew install poppler）
extract:
	@for f in data/raw/*.pdf; do \
		[ -e "$$f" ] || { echo "data/raw に PDF がありません"; exit 1; }; \
		echo "extract: $$f"; \
		pdftotext -layout "$$f" "$${f%.pdf}.txt"; \
	done

ingest:
	cd backend && go run ./cmd/ingest -dir ../data/raw

# 使い方: make search Q="要件テキスト" [K=5]
search:
	@test -n "$(Q)" || { echo 'Q="要件テキスト" を指定してください'; exit 1; }
	cd backend && go run ./cmd/search -k $(K) "$(Q)"

# 取り込み済みの見出し（section）を柱ごとに一覧表示する。評価ケースの期待値を決めるときに使う
sections:
	cd backend && go run ./cmd/sections

# API サーバー（要 ANTHROPIC_API_KEY / CLAUDE_MODEL）
server:
	cd backend && go run ./cmd/server

# フロント開発サーバー（:5173、/api は :8080 へ中継）
web:
	cd frontend && pnpm install && pnpm dev

# 検索評価（recall@k）。LABEL は改善前後の比較用。GENERATE=1 で出力品質も評価（Claude の課金あり）
evaluate:
	cd backend && go run ./cmd/eval -k $(K) -label "$(or $(LABEL),baseline)" $(if $(GENERATE),-generate)

vet:
	cd backend && go vet ./...

test:
	cd backend && go test ./...
