// @vitest-environment jsdom
//
// T119 — the half T110 scoped out: RouteErrorBoundary's RENDER path, which only a DOM can assert. T110
// covered getDerivedStateFromError (the DECISION to fail); this covers what the user actually SEES when a
// lazy route chunk fails to fetch — an honest, actionable error screen with a reload affordance — plus the
// RouteFallback loading state. e2e structurally can't reach this: a chunk-fetch failure isn't something the
// dev server produces on demand, so without this test a `render()` that returned null would pass every
// suite while a user on dropped Wi-Fi got the blank page T112 exists to prevent.
import { describe, it, expect, afterEach, vi } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { RouteErrorBoundary, RouteFallback } from "../src/components/RouteBoundary";

afterEach(cleanup);

/** A child that throws on render — the lazy-chunk-fetch failure, from the boundary's point of view. */
function Boom(): never {
  throw new Error("Failed to fetch dynamically imported module");
}

describe("RouteErrorBoundary render()", () => {
  it("renders its children unchanged when nothing throws", () => {
    render(
      <RouteErrorBoundary>
        <p>the real page</p>
      </RouteErrorBoundary>,
    );
    expect(screen.getByText("the real page")).toBeTruthy();
    expect(screen.queryByTestId("route-error")).toBeNull(); // no error screen on the happy path
  });

  it("a throwing child renders the honest, announced error screen — not a blank page", () => {
    // React logs the caught render error to console.error; silence it so the run output stays clean.
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <RouteErrorBoundary>
        <Boom />
      </RouteErrorBoundary>,
    );
    errSpy.mockRestore();

    // Announced to assistive tech, not a silent swap.
    expect(screen.getByTestId("route-error").getAttribute("role")).toBe("alert");
    // What the user READS: an honest heading + an actionable cause, not a stack trace or a blank screen.
    expect(screen.getByRole("heading", { name: /couldn.t load this page/i })).toBeTruthy();
    expect(screen.getByText(/dropped connection/i)).toBeTruthy();
    // The children are gone — the failed branch replaced them.
    expect(screen.queryByText("the real page")).toBeNull();
  });

  it("the reload affordance is present and actually reloads", () => {
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <RouteErrorBoundary>
        <Boom />
      </RouteErrorBoundary>,
    );
    errSpy.mockRestore();

    // jsdom's location.reload is a no-op stub; replace it so we can assert the click wires to it.
    const reload = vi.fn();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { ...window.location, reload },
    });

    fireEvent.click(screen.getByRole("button", { name: /reload/i }));
    expect(reload).toHaveBeenCalledTimes(1);
  });
});

describe("RouteFallback render()", () => {
  it("shows a loading state while a chunk is being fetched", () => {
    render(<RouteFallback />);
    expect(screen.getByTestId("route-fallback")).toBeTruthy();
    expect(screen.getByText(/loading/i)).toBeTruthy();
  });
});
