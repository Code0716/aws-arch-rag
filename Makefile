-include .env
export

K ?= 5

.PHONY: up down extract ingest search test vet

up:
	docker compose up -d --wait db
	docker compose run --rm migrate

down:
	docker compose down

# data/raw/*.pdf を pdftotext でテキスト化する（brew install poppler）
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

vet:
	cd backend && go vet ./...

test:
	cd backend && go test ./...
