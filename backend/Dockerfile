FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# cmd/ 配下（server, ingest, search, eval）をすべて /out/ に出力する
RUN CGO_ENABLED=0 go build -trimpath -o /out/ ./cmd/...

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ /app/
# 実行するバイナリは compose の entrypoint で選ぶ
ENTRYPOINT ["/app/server"]
