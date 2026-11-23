CREATE EXTENSION IF NOT EXISTS vector;

-- embedding の次元数は EMBED_DIM と一致させること（ingest 起動時に検証する）
CREATE TABLE chunks (
    id           BIGSERIAL PRIMARY KEY,
    source       TEXT         NOT NULL,
    pillar       TEXT         NOT NULL,
    section      TEXT         NOT NULL,
    content      TEXT         NOT NULL,
    content_hash TEXT         NOT NULL,
    embedding    vector(1024) NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chunks_source_section_hash_key UNIQUE (source, section, content_hash)
);

CREATE INDEX chunks_embedding_hnsw_idx
    ON chunks USING hnsw (embedding vector_cosine_ops);

CREATE INDEX chunks_pillar_idx ON chunks (pillar);
