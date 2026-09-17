import { useEffect, useState } from "react";
import { api } from "../../api";

/**
 * T175 — where the song editor's Back arrow goes.
 *
 * A song has more than one parent: the band's list, a setlist row, a bare URL. The arrow used to name one
 * of them in all three cases, so arriving from a setlist and pressing Back dropped you at the band (VLL).
 * The origin therefore travels with the reader, in the URL — `?from=setlist:<id>` — which survives a
 * reload and a shared link where router state does not, and mirrors what this same component already does
 * with `?file=` (⟨D1⟩).
 */

/** The destination and the words on it. They are produced together and never separately, because ⟨D3⟩'s
 *  one inviolable property is that the label and the destination cannot disagree. */
export type BackTarget = { to: string; label: string };

export const bandBack = (bandId: string): BackTarget => ({
  to: `/bands/${bandId}`,
  label: "Back to band",
});

/**
 * parseFrom reads the origin parameter. It returns a setlist id ONLY for a strictly-shaped value; anything
 * else is null and the caller falls back to the band.
 *
 * Strict on purpose. `?from=` is user-editable and shareable, so its content is chosen by whoever wrote the
 * link — this value reaches a router path, and the only safe thing to put there is something that has
 * already been proved to be an id. Length-bounded and character-bounded rules out a traversal (`../`), a
 * scheme (`javascript:`), and a payload dressed as an id.
 */
export function parseFrom(raw: string | null): string | null {
  if (!raw) return null;
  const m = /^setlist:([A-Za-z0-9-]{1,64})$/.exec(raw);
  return m ? m[1] : null;
}

/** The link a setlist row writes so the editor knows where the reader came from (⟨D4⟩: written at the
 *  link, because the destination cannot infer it). */
export const songHrefFromSetlist = (bandId: string, songId: string, setlistId: string) =>
  `/bands/${bandId}/songs/${songId}?from=setlist:${encodeURIComponent(setlistId)}`;

/**
 * useBackTarget resolves the arrow. It answers `bandBack` until it is SURE of anything else, which is what
 * keeps ⟨D3⟩'s property true at every instant rather than eventually — a pending resolution must not show
 * "Back to setlist" over a destination that may turn out to be the band.
 *
 * Resolution is membership in the viewer's OWN authorised setlist list. That is what makes a `?from=`
 * pointing at another band's setlist, or a deleted one, fall back silently: it is simply not in the list
 * this viewer is allowed to see. It is also why the label never names the setlist (⟨D2⟩) — nothing is
 * rendered from the parameter, so a crafted link cannot put words in the chrome or leak a name across
 * bands.
 *
 * The fetch happens ONLY when there is a `from` to resolve, so the ordinary case — opening a song from the
 * band list, or a bare URL — costs nothing.
 */
export function useBackTarget(bandId: string | undefined, from: string | null): BackTarget {
  const setlistId = parseFrom(from);
  const [resolved, setResolved] = useState<string | null>(null);

  useEffect(() => {
    setResolved(null);
    if (!bandId || !setlistId) return;
    let live = true;
    api
      .listSetlists(bandId)
      .then((sls) => {
        // Present in the list the server let this viewer have → real, visible, in this band.
        if (live && sls.some((s) => s.id === setlistId)) setResolved(setlistId);
      })
      .catch(() => {
        /* unreachable server, or no right to the list: the band is always a safe answer (⟨D3⟩) */
      });
    return () => {
      live = false;
    };
  }, [bandId, setlistId]);

  if (!bandId) return { to: "/bands", label: "Back to bands" };
  if (!resolved) return bandBack(bandId);
  return { to: `/bands/${bandId}/setlists/${resolved}`, label: "Back to setlist" };
}
