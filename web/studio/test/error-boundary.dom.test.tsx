// @vitest-environment jsdom
//
// T119 — the app-level render-crash boundary (T32), same silent-failure class as RouteErrorBoundary and
// equally out of e2e's reach (you can't force a React render crash against a live server). Without this,
// a `render()` that returned null would pass every suite while a component throwing mid-render blanked the
// whole app to a white screen — the "silently die" mode VLL called out. Asserts the user SEES a crash
// screen that NAMES what went wrong, plus a reload affordance.
import { describe, it, expect, afterEach, vi } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { ErrorBoundary } from "../src/components/ErrorBoundary";

afterEach(cleanup);

function Boom(): never {
  throw new Error("boom in render");
}

describe("ErrorBoundary render()", () => {
  it("renders its children unchanged when nothing throws", () => {
    render(
      <ErrorBoundary>
        <span>the app</span>
      </ErrorBoundary>,
    );
    expect(screen.getByText("the app")).toBeTruthy();
    expect(screen.queryByTestId("render-crash")).toBeNull();
  });

  it("a render crash shows a visible, announced screen that NAMES the failure — not a white page", () => {
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );
    errSpy.mockRestore();

    expect(screen.getByTestId("render-crash").getAttribute("role")).toBe("alert");
    expect(screen.getByRole("heading", { name: /something went wrong/i })).toBeTruthy();
    expect(screen.getByText("boom in render")).toBeTruthy(); // the message surfaces, not swallowed
    expect(screen.queryByText("the app")).toBeNull(); // children replaced
  });

  it("offers a working reload", () => {
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );
    errSpy.mockRestore();

    const reload = vi.fn();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { ...window.location, reload },
    });
    fireEvent.click(screen.getByRole("button", { name: /reload/i }));
    expect(reload).toHaveBeenCalledTimes(1);
  });
});
