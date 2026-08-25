/**
 * T112 — the loading + failure UI for lazily-loaded route chunks. A lazy boundary is a new failure mode:
 * if the editor chunk can't be fetched (a dropped connection mid-navigation on band-practice Wi-Fi), the
 * app must not go blank. RouteFallback covers the pending fetch; RouteErrorBoundary covers the failed one
 * with an honest, actionable message.
 */
import { Component, type ReactNode } from "react";

/** Shown while a lazily-loaded route chunk is being fetched. */
export function RouteFallback() {
  return (
    <div className="route-fallback" data-testid="route-fallback">
      <p className="muted">Loading…</p>
    </div>
  );
}

export class RouteErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    if (this.state.failed) {
      return (
        <div className="route-error" data-testid="route-error" role="alert">
          <h2>Couldn’t load this page</h2>
          <p className="muted">
            Part of the app failed to download — usually a dropped connection. Reload to try again.
          </p>
          <button type="button" className="primary" onClick={() => window.location.reload()}>
            Reload
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
