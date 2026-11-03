// @vitest-environment jsdom
import { afterEach, describe, it, expect, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { DownloadPdfDialog } from "../src/pages/song-editor/DownloadPdfDialog";
import type { AnnotationLayer, SongFile } from "../src/api";

const ME = "u-me";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function layer(p: Partial<AnnotationLayer> & { id: string }): AnnotationLayer {
  return {
    fileId: "f1",
    name: p.id,
    ownerId: "u-other",
    zone: "personal",
    order: 0,
    access: "rw",
    mandatory: false,
    roleTag: "",
    ...p,
  };
}

// The spec's six layers for ONE file. defaultVisibility ticks: shared (incl. role-tagged), conductor,
// my own personal, mandatory. It does NOT tick another member's personal. The song-level layer has no
// fileId, so it is not a layer of this file at all.
const LAYERS: AnnotationLayer[] = [
  layer({ id: "shared-role", zone: "shared", ownerId: "u-cond", roleTag: "bass", name: "Bass cues", order: 0 }),
  layer({ id: "conductor", zone: "conductor", ownerId: "u-cond", name: "Form", order: 1 }),
  layer({ id: "mine", zone: "personal", ownerId: ME, name: "My notes", order: 2 }),
  layer({ id: "theirs", zone: "personal", ownerId: "u-other", name: "Their notes", order: 3 }),
  layer({ id: "must", zone: "shared", ownerId: "u-cond", mandatory: true, name: "Set list", order: 4 }),
  layer({ id: "song", fileId: "", zone: "shared", ownerId: "u-cond", name: "Song-level", order: 5 }),
];

const FILE: SongFile = {
  id: "f1",
  filename: "Lead sheet",
  contentType: "application/pdf",
  size: 10,
  displayOrder: 0,
  createdAt: "2026-01-01T00:00:00Z",
  revision: 3,
};

/** Install a fetch stub that records every request URL and answers with a tiny PDF. */
function stubFetch(): string[] {
  const urls: string[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    urls.push(String(input));
    return new Response(new Blob(["%PDF-1.4"]), {
      status: 200,
      headers: { "Content-Type": "application/pdf", "Content-Disposition": `attachment; filename="Song - Lead sheet.pdf"` },
    });
  });
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:x"), revokeObjectURL: vi.fn() }));
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  return urls;
}

function layersOf(url: string): { present: boolean; ids: string[] } {
  const u = new URL(url, "http://localhost");
  const raw = u.searchParams.get("layers");
  return { present: raw !== null, ids: raw ? raw.split(",").filter(Boolean) : [] };
}

describe("T208 — the Download PDF dialog's layer checklist", () => {
  it("lists exactly THIS file's layers, ticked as defaultVisibility says, mandatory locked", () => {
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={LAYERS} myUserId={ME} onClose={() => {}} />);
    const rows = screen.getAllByTestId("download-layer");
    // The song-level layer (no fileId) is absent; the other five are present, in order.
    expect(rows.map((r) => r.getAttribute("data-layer"))).toEqual(["shared-role", "conductor", "mine", "theirs", "must"]);

    const state = Object.fromEntries(
      rows.map((r) => {
        const cb = within(r).getByTestId("download-layer-toggle") as HTMLInputElement;
        return [r.getAttribute("data-layer"), { checked: cb.checked, disabled: cb.disabled }];
      }),
    );
    expect(state["shared-role"]).toEqual({ checked: true, disabled: false });
    expect(state["conductor"]).toEqual({ checked: true, disabled: false });
    expect(state["mine"]).toEqual({ checked: true, disabled: false });
    expect(state["theirs"]).toEqual({ checked: false, disabled: false });
    // mandatory: ticked and locked, marked "always printed".
    expect(state["must"]).toEqual({ checked: true, disabled: true });
    expect(within(rows[4]).getByTestId("download-always").textContent).toBe("always printed");
  });

  it("Download sends layers= with exactly the ticked ids — the role-tagged shared one INCLUDED", async () => {
    const urls = stubFetch();
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={LAYERS} myUserId={ME} onClose={() => {}} />);
    // Default choice is "With annotations"; the defaults match defaultVisibility.
    fireEvent.click(screen.getByTestId("download-dialog-confirm"));
    await waitFor(() => expect(urls.length).toBe(1));

    const { present, ids } = layersOf(urls[0]);
    // THE DISCRIMINATING VECTOR: the server's absent-param default (role "") would DROP the role-tagged
    // shared layer that the dialog shows. So the parameter must be present AND carry its id.
    expect(present).toBe(true);
    expect(ids).toContain("shared-role");
    // exactly the ticked ids: role-tagged shared + conductor + my own — not another member's, not the
    // song-level layer, and not the mandatory one (the server forces that on by itself).
    expect(new Set(ids)).toEqual(new Set(["shared-role", "conductor", "mine"]));
    expect(ids).not.toContain("theirs");
    expect(ids).not.toContain("song");
    expect(ids).not.toContain("must");
  });

  it("unticking everything still sends layers= with an EMPTY value, never no parameter", async () => {
    const urls = stubFetch();
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={LAYERS} myUserId={ME} onClose={() => {}} />);
    for (const id of ["shared-role", "conductor", "mine"]) {
      const row = screen.getByTestId("download-layer-list").querySelector(`[data-layer="${id}"] input`)!;
      fireEvent.click(row);
    }
    fireEvent.click(screen.getByTestId("download-dialog-confirm"));
    await waitFor(() => expect(urls.length).toBe(1));

    const { present, ids } = layersOf(urls[0]);
    expect(present).toBe(true); // the parameter is THERE…
    expect(ids).toEqual([]); // …with an empty value. Omitting it would be the server's default view.
  });
});

describe("T208 — a file with no annotations", () => {
  it("disables 'With annotations' with its line and selects 'Without annotations'", () => {
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={[]} myUserId={ME} onClose={() => {}} />);
    expect((screen.getByTestId("download-choice-with") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByTestId("download-choice-without") as HTMLInputElement).checked).toBe(true);
    expect(screen.getByTestId("download-no-annotations").textContent).toBe("This file has no annotations.");
    expect(screen.queryByTestId("download-layer")).toBeNull();
  });
});

describe("T208 — Escape and focus", () => {
  it("moves focus into the dialog (the Cancel button) on open", () => {
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={LAYERS} myUserId={ME} onClose={() => {}} />);
    expect(document.activeElement).toBe(screen.getByTestId("download-dialog-cancel"));
  });

  it("Escape closes the dialog when idle", () => {
    const onClose = vi.fn();
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={LAYERS} myUserId={ME} onClose={onClose} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("Escape does NOTHING while composing (Preparing PDF…)", async () => {
    const onClose = vi.fn();
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {}))); // never resolves → stays busy
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={LAYERS} myUserId={ME} onClose={onClose} />);
    const btn = screen.getByTestId("download-dialog-confirm");
    fireEvent.click(btn); // choice defaults to "with" → enters the busy state
    await waitFor(() => expect(btn.textContent).toBe("Preparing PDF…"));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("T208 — Without annotations", () => {
  it("downloads the stored file for the current revision (no layers endpoint)", async () => {
    const urls = stubFetch();
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={LAYERS} myUserId={ME} onClose={() => {}} />);
    fireEvent.click(screen.getByTestId("download-choice-without"));
    fireEvent.click(screen.getByTestId("download-dialog-confirm"));
    await waitFor(() => expect(urls.length).toBe(1));
    // The stored-file endpoint, pinned to the current revision — not the /pdf compositor.
    expect(urls[0]).toBe("/api/files/f1?rev=3");
  });

  it("shows the busy label while composing and the server's error in the banner", async () => {
    let release!: (r: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((res) => (release = res))));
    render(<DownloadPdfDialog bandId="B" songId="S" file={FILE} layers={LAYERS} myUserId={ME} onClose={() => {}} />);
    const btn = screen.getByTestId("download-dialog-confirm") as HTMLButtonElement;
    fireEvent.click(btn);
    expect(btn.disabled).toBe(true);
    expect(btn.textContent).toBe("Preparing PDF…");
    release(new Response(JSON.stringify({ error: "app: forbidden" }), { status: 403, headers: { "Content-Type": "application/json" } }));
    const banner = await screen.findByTestId("error");
    expect(banner.textContent).toBe("app: forbidden");
    expect(btn.disabled).toBe(false); // dialog stays open, retryable
  });
});
