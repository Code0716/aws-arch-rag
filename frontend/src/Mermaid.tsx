import { useEffect, useId, useState } from "react";

type Props = { code: string };

/** Mermaid を SVG で描画する。構文エラー等で失敗したら、コードをそのまま表示する。 */
export function Mermaid({ code }: Props) {
  const id = "mmd-" + useId().replace(/:/g, "");
  const [svg, setSvg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setSvg(null);
    setError(null);
    (async () => {
      try {
        const { default: mermaid } = await import("mermaid");
        mermaid.initialize({ startOnLoad: false, securityLevel: "strict" });
        const out = await mermaid.render(id, code);
        if (!cancelled) setSvg(out.svg);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [code, id]);

  if (error) {
    return (
      <div className="mermaid-fallback">
        <p className="muted">図を描画できなかったため、Mermaid のコードを表示しています。</p>
        <pre>{code}</pre>
      </div>
    );
  }
  if (!svg) return <p className="muted">図を描画中…</p>;
  // securityLevel: "strict" で Mermaid がサニタイズした SVG を埋め込む
  return <div className="mermaid" dangerouslySetInnerHTML={{ __html: svg }} />;
}
