export type SSEEvent = { event: string; data: string };

/**
 * Server-Sent Events の逐次パーサー。
 * fetch の ReadableStream から来るチャンクを push() に渡すと、
 * 空行で区切られた完成済みのイベントごとに onEvent が呼ばれる。
 */
export function createSSEParser(onEvent: (e: SSEEvent) => void) {
  let buffer = "";

  const dispatch = (block: string) => {
    let event = "message";
    const data: string[] = [];
    for (const line of block.split("\n")) {
      if (line === "" || line.startsWith(":")) continue;
      const i = line.indexOf(":");
      const field = i < 0 ? line : line.slice(0, i);
      let value = i < 0 ? "" : line.slice(i + 1);
      if (value.startsWith(" ")) value = value.slice(1);
      if (field === "event") event = value;
      else if (field === "data") data.push(value);
    }
    if (data.length > 0) onEvent({ event, data: data.join("\n") });
  };

  return {
    push(chunk: string) {
      buffer += chunk.replace(/\r\n/g, "\n");
      let idx: number;
      while ((idx = buffer.indexOf("\n\n")) >= 0) {
        dispatch(buffer.slice(0, idx));
        buffer = buffer.slice(idx + 2);
      }
    },
  };
}
