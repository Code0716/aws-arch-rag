import { useEffect, useState } from "react";
import { getChunk } from "./api";
import type { Chunk } from "./types";

type Props = { chunkId: number | null; onClose: () => void };

/** 引用をクリックしたときに、元のチャンクを横に表示するパネル。 */
export function CitationPanel({ chunkId, onClose }: Props) {
  const [chunk, setChunk] = useState<Chunk | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (chunkId === null) return;
    let cancelled = false;
    setChunk(null);
    setError(null);
    getChunk(chunkId)
      .then((c) => !cancelled && setChunk(c))
      .catch((e: Error) => !cancelled && setError(e.message));
    return () => {
      cancelled = true;
    };
  }, [chunkId]);

  if (chunkId === null) return <aside className="panel muted">引用 [n] をクリックすると、元の文書の該当箇所をここに表示します。</aside>;

  return (
    <aside className="panel">
      <button className="close" onClick={onClose} aria-label="閉じる">
        ×
      </button>
      {error && <p className="error">{error}</p>}
      {!chunk && !error && <p className="muted">読み込み中…</p>}
      {chunk && (
        <>
          <h3>{chunk.section}</h3>
          <p className="muted">
            {chunk.pillar} / {chunk.source} / chunk {chunk.id}
          </p>
          <p className="chunk-body">{chunk.content}</p>
        </>
      )}
    </aside>
  );
}
