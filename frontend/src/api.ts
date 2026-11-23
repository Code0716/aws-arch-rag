import { createSSEParser } from "./sse";
import type { Chunk, HitMeta, Proposal } from "./types";

export type ProposeHandlers = {
  onRetrieved: (hits: HitMeta[]) => void;
  onDelta: (text: string) => void;
  onResult: (p: Proposal) => void;
  onError: (message: string) => void;
};

/** POST /api/propose を叩き、SSE のイベントをハンドラに流す。 */
export async function propose(requirements: string, h: ProposeHandlers, signal?: AbortSignal): Promise<void> {
  const res = await fetch("/api/propose", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ requirements }),
    signal,
  });
  if (!res.ok || !res.body) {
    h.onError(`リクエストに失敗しました (HTTP ${res.status})`);
    return;
  }

  const parser = createSSEParser(({ event, data }) => {
    const payload = JSON.parse(data);
    if (event === "retrieved") h.onRetrieved(payload);
    else if (event === "delta") h.onDelta(payload.text);
    else if (event === "result") h.onResult(payload);
    else if (event === "error") h.onError(payload.message);
  });

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    parser.push(decoder.decode(value, { stream: true }));
  }
}

/** 引用元チャンクを取得する。 */
export async function getChunk(id: number): Promise<Chunk> {
  const res = await fetch(`/api/chunks/${id}`);
  if (!res.ok) throw new Error(`チャンク ${id} を取得できませんでした (HTTP ${res.status})`);
  return res.json();
}
