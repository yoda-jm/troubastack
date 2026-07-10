import { Component, type ReactNode } from "react";

/**
 * Render-crash boundary (T32). A component throwing DURING RENDER would otherwise
 * blank the whole app to a white screen — the "silently die" failure mode VLL called
 * out. Catch it and show the message plus a reload hint, so a crash is always visible.
 * (Runtime/async errors outside render are caught by the global backstop instead.)
 */
export class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state: { error: Error | null } = { error: null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error) {
    console.error("render crash", error);
  }

  render() {
    if (this.state.error) {
      return (
        <div role="alert" data-testid="render-crash" className="crash-screen">
          <h2>Something went wrong.</h2>
          <p className="crash-message">{this.state.error.message}</p>
          <button type="button" onClick={() => window.location.reload()}>
            Reload
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
