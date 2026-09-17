import { describe, it, expect } from "vitest";
import { bandBack, parseFrom, songHrefFromSetlist } from "../src/pages/song-editor/backTarget";

describe("T175 — parsing the origin", () => {
  it("accepts a well-formed setlist origin", () => {
    expect(parseFrom("setlist:8a80bf7f-6f2b-4d48-a8bb-18b98290cb55")).toBe(
      "8a80bf7f-6f2b-4d48-a8bb-18b98290cb55",
    );
  });

  it("returns null for anything that is not one", () => {
    for (const raw of [null, "", "band:abc", "setlist:", "setlist", "8a80bf7f", "SETLIST:abc"]) {
      expect(parseFrom(raw), `parseFrom(${JSON.stringify(raw)}) should be null`).toBeNull();
    }
  });

  // `?from=` is user-editable and shareable, so its content is chosen by whoever wrote the link, and the
  // value reaches a ROUTER PATH. Everything here is rejected by shape, before it can mean anything: a
  // traversal that would climb out of the band, a scheme, a space-separated payload, an over-long token.
  it("refuses anything that could mean something other than an id", () => {
    for (const evil of [
      "setlist:../../bands/other",
      "setlist:..%2F..%2Fadmin",
      "setlist:javascript:alert(1)",
      "setlist:a b",
      "setlist:<script>",
      "setlist:" + "a".repeat(65),
      "setlist:abc?x=1",
      "setlist:abc#frag",
      "setlist:abc/def",
    ]) {
      expect(parseFrom(evil), `parseFrom(${JSON.stringify(evil)}) must be null`).toBeNull();
    }
  });

  it("the link a setlist row writes round-trips through the parser", () => {
    const href = songHrefFromSetlist("b1", "s1", "sl-42");
    expect(href).toBe("/bands/b1/songs/s1?from=setlist:sl-42");
    expect(parseFrom(new URL(href, "http://x").searchParams.get("from"))).toBe("sl-42");
  });

  it("the band fallback names the band and goes to it", () => {
    // ⟨D3⟩'s inviolable property, at the one place both halves are produced.
    const b = bandBack("b1");
    expect(b.to).toBe("/bands/b1");
    expect(b.label).toBe("Back to band");
    expect(b.label.toLowerCase()).toContain("band");
    expect(b.label.toLowerCase()).not.toContain("setlist");
  });
});
