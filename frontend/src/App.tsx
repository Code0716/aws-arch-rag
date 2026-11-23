import { useRef, useState } from "react";
import { propose } from "./api";
import { CitationPanel } from "./CitationPanel";
import { Mermaid } from "./Mermaid";
import type { HitMeta, Proposal } from "./types";

const EXAMPLE = "月間10万PVのECサイト。商品画像のアップロードあり。可用性を重視、予算は抑えたい。";

export function App() {
  const [requirements, setRequirements] = useState(EXAMPLE);
  const [loading, setLoading] = useState(false);
  const [received, setReceived] = useState(0);
  const [hits, setHits] = useState<HitMeta[]>([]);
  const [result, setResult] = useState<Proposal | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [citation, setCitation] = useState<number | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  async function submit() {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;

    setLoading(true);
    setReceived(0);
    setHits([]);
    setResult(null);
    setError(null);
    setCitation(null);
    try {
      await propose(
        requirements,
        {
          onRetrieved: setHits,
          onDelta: (chars) => setReceived((n) => n + chars),
          onResult: setResult,
          onError: setError,
        },
        ctrl.signal,
      );
    } catch (e) {
      if (!ctrl.signal.aborted) setError(e instanceof Error ? e.message : String(e));
    } finally {
      if (abortRef.current === ctrl) setLoading(false);
    }
  }

  return (
    <div className="layout">
      <main>
        <h1>AWS構成案 × Well-Architected</h1>
        <textarea
          value={requirements}
          onChange={(e) => setRequirements(e.target.value)}
          rows={4}
          placeholder="システムの要件を入力"
        />
        <button onClick={submit} disabled={loading || requirements.trim() === ""}>
          {loading ? "生成中…" : "構成案を作る"}
        </button>

        {loading && !result && (
          <p className="muted">
            {hits.length > 0 ? `${hits.length}件の関連箇所を取得。` : ""}
            生成中… ({received}文字)
          </p>
        )}
        {error && <p className="error">{error}</p>}

        {result && (
          <>
            <section>
              <h2>構成案</h2>
              <p>{result.summary}</p>
              <Mermaid code={result.mermaid} />
              <ul>
                {result.components.map((c) => (
                  <li key={c.name}>
                    <strong>{c.name}</strong>: {c.role}
                  </li>
                ))}
              </ul>
            </section>

            <section>
              <h2>柱ごとの根拠</h2>
              {result.rationale.map((r, i) => (
                <div key={i} className="rationale">
                  <span className="pillar">{r.pillar}</span>
                  <p>
                    {r.text}{" "}
                    {r.chunk_ids.map((id) => (
                      <button key={id} className="cite" onClick={() => setCitation(id)}>
                        [{id}]
                      </button>
                    ))}
                  </p>
                </div>
              ))}
            </section>

            <section>
              <h2>懸念点</h2>
              <ul>
                {result.concerns.map((c, i) => (
                  <li key={i}>{c}</li>
                ))}
              </ul>
            </section>

            <section>
              <h2>追加で確認したい質問</h2>
              <ul>
                {result.questions.map((q, i) => (
                  <li key={i}>{q}</li>
                ))}
              </ul>
            </section>
          </>
        )}
      </main>
      <CitationPanel chunkId={citation} onClose={() => setCitation(null)} />
    </div>
  );
}
