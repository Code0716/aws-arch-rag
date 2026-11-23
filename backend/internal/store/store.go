// Package store は pgvector への読み書きを担う。
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

type Store struct {
	pool *pgxpool.Pool
}

// Row は保存する1チャンク。
type Row struct {
	Source    string
	Pillar    string
	Section   string
	Content   string
	Hash      string
	Embedding []float32
}

// Hit は検索結果1件。Score は cosine 類似度（1 - cosine 距離）。
type Hit struct {
	ID      int64
	Score   float64
	Source  string
	Pillar  string
	Section string
	Content string
}

func New(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, c)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect db (migrate 済みか確認してください): %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// EmbeddingDim は chunks.embedding の次元数（vector(N) の N）を返す。
func (s *Store) EmbeddingDim(ctx context.Context) (int, error) {
	var dim int
	err := s.pool.QueryRow(ctx, `
		SELECT atttypmod FROM pg_attribute
		WHERE attrelid = 'chunks'::regclass AND attname = 'embedding'`).Scan(&dim)
	return dim, err
}

// Existing は source に登録済みの (section, hash) 集合を返す。キーは ExistingKey で作る。
func (s *Store) Existing(ctx context.Context, source string) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT section, content_hash FROM chunks WHERE source = $1`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var section, hash string
		if err := rows.Scan(&section, &hash); err != nil {
			return nil, err
		}
		out[ExistingKey(section, hash)] = true
	}
	return out, rows.Err()
}

func ExistingKey(section, hash string) string { return section + "\x00" + hash }

// Insert は行を保存し、実際に挿入された件数を返す（重複は無視）。
func (s *Store) Insert(ctx context.Context, rows []Row) (int, error) {
	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(`
			INSERT INTO chunks (source, pillar, section, content, content_hash, embedding)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (source, section, content_hash) DO NOTHING`,
			r.Source, r.Pillar, r.Section, r.Content, r.Hash, pgvector.NewVector(r.Embedding))
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	inserted := 0
	for range rows {
		tag, err := br.Exec()
		if err != nil {
			return inserted, err
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}

// Chunk は引用元表示用の1チャンク。
type Chunk struct {
	ID      int64  `json:"id"`
	Source  string `json:"source"`
	Pillar  string `json:"pillar"`
	Section string `json:"section"`
	Content string `json:"content"`
}

// ErrNotFound は指定 ID のチャンクが存在しない場合のエラー。
var ErrNotFound = errors.New("chunk not found")

// Get は ID でチャンクを1件取得する。
func (s *Store) Get(ctx context.Context, id int64) (Chunk, error) {
	var c Chunk
	err := s.pool.QueryRow(ctx,
		`SELECT id, source, pillar, section, content FROM chunks WHERE id = $1`, id).
		Scan(&c.ID, &c.Source, &c.Pillar, &c.Section, &c.Content)
	if errors.Is(err, pgx.ErrNoRows) {
		return Chunk{}, ErrNotFound
	}
	return c, err
}

// Search は cosine 距離の近い順に上位 k 件を返す（HNSW インデックスを使う）。
func (s *Store) Search(ctx context.Context, vec []float32, k int) ([]Hit, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, 1 - (embedding <=> $1) AS score, source, pillar, section, content
		FROM chunks
		ORDER BY embedding <=> $1
		LIMIT $2`, pgvector.NewVector(vec), k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.ID, &h.Score, &h.Source, &h.Pillar, &h.Section, &h.Content); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}
