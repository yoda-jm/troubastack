// WIN08 — initShellBridge accepts two shell shapes. Android (and anything else) sets the object
// `window.TroubaStageShell`; the Windows SWT host can only register a BrowserFunction whose name is a
// bare identifier, so it exposes `window.TroubaStageShellReceive` and we wrap it. This proves the three
// vectors (function-only, object-only, neither) and that dropping the new branch kills the function path.
import { describe, it, expect, beforeEach, vi } from "vitest";
import { initShellBridge } from "../src/bridge";

type Win = {
  TroubaStageShell?: { receive(json: string): void };
  TroubaStageShellReceive?: (json: string) => void;
  __troubastageShell?: { deliver(json: string): void };
};

function withWindow(w: Win): Win {
  (globalThis as { window?: unknown }).window = w;
  return w;
}

beforeEach(() => {
  delete (globalThis as { window?: unknown }).window;
});

describe("initShellBridge shell shapes (WIN08)", () => {
  it("function-only: ready goes out through TroubaStageShellReceive, and hello is answered", () => {
    const receive = vi.fn();
    const w = withWindow({ TroubaStageShellReceive: receive });

    initShellBridge();

    // Proactive ready reached the function-shaped shell.
    expect(receive).toHaveBeenCalledWith(JSON.stringify({ type: "ready" }));
    expect(receive).toHaveBeenCalledTimes(1);
    // The native→web deliver channel is wired, and a hello is answered with ready through the same shell.
    expect(typeof w.__troubastageShell?.deliver).toBe("function");
    w.__troubastageShell!.deliver(JSON.stringify({ type: "hello" }));
    expect(receive).toHaveBeenCalledTimes(2);
  });

  it("object-only: ready goes out through TroubaStageShell.receive (Android path, unchanged)", () => {
    const receive = vi.fn();
    const w = withWindow({ TroubaStageShell: { receive } });

    initShellBridge();

    expect(receive).toHaveBeenCalledWith(JSON.stringify({ type: "ready" }));
    expect(receive).toHaveBeenCalledTimes(1);
    expect(typeof w.__troubastageShell?.deliver).toBe("function");
  });

  it("the object form wins when both are present (never double-wire)", () => {
    const objReceive = vi.fn();
    const fnReceive = vi.fn();
    withWindow({ TroubaStageShell: { receive: objReceive }, TroubaStageShellReceive: fnReceive });

    initShellBridge();

    expect(objReceive).toHaveBeenCalledTimes(1);
    expect(fnReceive).not.toHaveBeenCalled();
  });

  it("neither: inert — no deliver channel, no throw (plain browser, I10)", () => {
    const w = withWindow({});

    expect(() => initShellBridge()).not.toThrow();
    expect(w.__troubastageShell).toBeUndefined();
  });
});
