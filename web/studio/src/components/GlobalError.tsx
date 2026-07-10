import { useEffect, useState } from "react";

/**
 * Global client-error backstop (T32; VLL directive 2026-07-10: "this kind of error
 * needs to be caught and made visible to the user; it is not normal to just die
 * silently"). Surfaces ANY uncaught error or unhandled promise rejection as a
 * dismissible banner — the safety net beneath the targeted commit-path notices, so a
 * bug like the insecure-context `crypto.randomUUID` throw can never again fail with
 * "no error surface". Keeps only the latest message (no queue, no reporting service).
 */
export function GlobalError() {
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    const onError = (e: ErrorEvent) => setMessage(e.message || "Unexpected error");
    const onRejection = (e: PromiseRejectionEvent) => {
      const r = e.reason;
      setMessage(r instanceof Error ? r.message : String(r ?? "Unhandled promise rejection"));
    };
    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onRejection);
    return () => {
      window.removeEventListener("error", onError);
      window.removeEventListener("unhandledrejection", onRejection);
    };
  }, []);

  if (!message) return null;
  return (
    <div role="alert" data-testid="global-error" className="global-error">
      <span className="global-error-msg">{message}</span>
      <button
        type="button"
        data-testid="global-error-dismiss"
        className="global-error-dismiss"
        aria-label="Dismiss error"
        onClick={() => setMessage(null)}
      >
        ×
      </button>
    </div>
  );
}
