// @vitest-environment jsdom
import { afterEach, describe, it, expect, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { FilePdfButton, visibleLayerIdsForFile } from "../src/pages/song-editor/FilePdfButton";
import { filenameFromDisposition, type AnnotationLayer } from "../src/api";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function layer(id: string, fileId: string): AnnotationLayer {
  return { id, fileId } as AnnotationLayer;
}

const LAYERS = [layer("a", "f1"), layer("b", "f1"), layer("c", "f2"), layer("song", "")];

describe("FILEPDF — which layers a file print asks for", () => {
  it("sends exactly the visible layers of THIS file", () => {
    const vis = { a: true, b: false, c: true, song: true };
    expect(visibleLayerIdsForFile(LAYERS, vis, "f1")).toEqual(["a"]);
    expect(visibleLayerIdsForFile(LAYERS, vis, "f2")).toEqual(["c"]);
  });
  it("is empty when nothing of the file is shown (the server then adds mandatory only)", () => {
    expect(visibleLayerIdsForFile(LAYERS, {}, "f1")).toEqual([]);
  });
});

describe("FILEPDF — the download name", () => {
  it("prefers the exact UTF-8 filename* over the ASCII fallback", () => {
    expect(
      filenameFromDisposition(`attachment; filename="Cafe - Lead.pdf"; filename*=UTF-8''Caf%C3%A9%20-%20Lead.pdf`, "x.pdf"),
    ).toBe("Café - Lead.pdf");
  });
  it("falls back to the quoted ASCII name, then to the given default", () => {
    expect(filenameFromDisposition(`attachment; filename="Song - Tab.pdf"`, "x.pdf")).toBe("Song - Tab.pdf");
    expect(filenameFromDisposition(null, "file.pdf")).toBe("file.pdf");
  });
});

describe("FILEPDF — the button", () => {
  it("requests the visible layers, shows a busy label, and saves the server's filename", async () => {
    let release!: (r: Response) => void;
    const fetchMock = vi.fn(() => new Promise<Response>((res) => (release = res)));
    vi.stubGlobal("fetch", fetchMock);
    const createURL = vi.fn(() => "blob:x");
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: createURL, revokeObjectURL: vi.fn() }));
    let savedAs = "";
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      savedAs = this.download;
    });

    render(<FilePdfButton bandId="B" songId="S" fileId="f1" layers={LAYERS} visible={{ a: true, b: false, c: true }} />);
    const btn = screen.getByTestId("file-pdf-download");
    expect(btn.getAttribute("title")).toBe("Download this file as a PDF, with the layers shown on screen");
    fireEvent.click(btn);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(String((fetchMock.mock.calls[0] as unknown[])[0])).toBe("/api/bands/B/songs/S/files/f1/pdf?layers=a");
    expect((btn as HTMLButtonElement).disabled).toBe(true);
    expect(btn.textContent).toBe("Preparing PDF…");

    release(
      new Response(new Blob(["%PDF-1.4"]), {
        status: 200,
        headers: { "Content-Type": "application/pdf", "Content-Disposition": `attachment; filename="S - f.pdf"` },
      }),
    );
    await waitFor(() => expect((btn as HTMLButtonElement).disabled).toBe(false));
    expect(btn.textContent).toBe("Download PDF");
    expect(createURL).toHaveBeenCalledTimes(1);
    expect(savedAs).toBe("S - f.pdf");
  });

  it("an empty visible set is still sent (layers=), never omitted", async () => {
    const fetchMock = vi.fn(async () => new Response(new Blob(["%PDF"]), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:x"), revokeObjectURL: vi.fn() }));
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    render(<FilePdfButton bandId="B" songId="S" fileId="f1" layers={LAYERS} visible={{}} />);
    fireEvent.click(screen.getByTestId("file-pdf-download"));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(String((fetchMock.mock.calls[0] as unknown[])[0])).toMatch(/\/pdf\?layers=$/);
  });

  it("shows the server's error in the error banner", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ error: "app: forbidden" }), { status: 403, headers: { "Content-Type": "application/json" } })),
    );
    render(<FilePdfButton bandId="B" songId="S" fileId="f1" layers={LAYERS} visible={{ a: true }} />);
    fireEvent.click(screen.getByTestId("file-pdf-download"));
    const banner = await screen.findByTestId("error");
    expect(banner.textContent).toBe("app: forbidden");
    expect((screen.getByTestId("file-pdf-download") as HTMLButtonElement).disabled).toBe(false);
  });
});
