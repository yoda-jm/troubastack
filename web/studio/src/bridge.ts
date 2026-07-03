/**
 * Thin, feature-detected bridge to a native shell (the TroubaShare app hosting Studio in a WebView).
 * In a plain browser there is no shell, so every function here is inert — pure-browser Studio is
 * completely unaffected (I10). The only traffic today is a handshake; the real client is the native
 * ink overlay (A07, blocked), for which `postToShell` is a ready no-op-safe helper.
 */

interface ShellPort {
  /** The shell receives a JSON string from Studio (Android: an @JavascriptInterface method). */
  receive(json: string): void;
}

declare global {
  interface Window {
    TroubaShareShell?: ShellPort;
    __troubashareShell?: { deliver(json: string): void };
  }
}

/** Call once at startup. Does nothing in a browser; wires the handshake when hosted by the shell. */
export function initShellBridge(): void {
  const shell = window.TroubaShareShell;
  if (!shell) return; // pure browser — no shell present (I10)

  // shell → web: the shell calls window.__troubashareShell.deliver(json).
  window.__troubashareShell = {
    deliver(json: string) {
      try {
        const msg = JSON.parse(json) as { type?: string };
        if (msg?.type === "hello") shell.receive(JSON.stringify({ type: "ready" }));
      } catch {
        /* ignore malformed shell messages */
      }
    },
  };

  // Proactively announce readiness too, so the handshake completes regardless of message ordering.
  shell.receive(JSON.stringify({ type: "ready" }));
}

/** Send a JSON string to the native shell. No-op in a browser (feature-detected). */
export function postToShell(json: string): void {
  window.TroubaShareShell?.receive(json);
}
