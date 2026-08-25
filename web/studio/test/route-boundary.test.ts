// T110 (per T112 review) — the one component case: the error boundary's derive-state must flip to failed
// so a broken lazy-chunk fetch renders the honest error screen, not a blank page. Pure static method.
import { describe, it, expect } from "vitest";
import { RouteErrorBoundary } from "../src/components/RouteBoundary";

describe("RouteErrorBoundary.getDerivedStateFromError", () => {
  it("flips to { failed: true }", () => {
    expect(RouteErrorBoundary.getDerivedStateFromError()).toEqual({ failed: true });
  });
});
