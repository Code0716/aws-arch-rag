import { describe, expect, it } from "vitest";
import { createSSEParser, type SSEEvent } from "./sse";

function collect() {
  const events: SSEEvent[] = [];
  return { events, parser: createSSEParser((e) => events.push(e)) };
}

describe("createSSEParser", () => {
  it("イベントを種類とデータに分ける", () => {
    const { events, parser } = collect();
    parser.push('event: retrieved\ndata: [{"id":1}]\n\nevent: delta\ndata: {"chars":1}\n\n');
    expect(events).toEqual([
      { event: "retrieved", data: '[{"id":1}]' },
      { event: "delta", data: '{"chars":1}' },
    ]);
  });

  it("チャンク境界が途中でも、完成してから通知する", () => {
    const { events, parser } = collect();
    parser.push("event: res");
    parser.push('ult\ndata: {"a"');
    expect(events).toHaveLength(0);
    parser.push(":1}\n\n");
    expect(events).toEqual([{ event: "result", data: '{"a":1}' }]);
  });

  it("CRLF とコメント行を扱える", () => {
    const { events, parser } = collect();
    parser.push(": keep-alive\r\n\r\nevent: error\r\ndata: x\r\n\r\n");
    expect(events).toEqual([{ event: "error", data: "x" }]);
  });
});
