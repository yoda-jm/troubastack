/**
 * T175 — where the song editor's Back arrow goes.
 *
 * A song has more than one parent: the band's list, a setlist row, a bare URL. The arrow used to name one
 * of them in all three cases, so arriving from a setlist and pressing Back dropped you at the band (VLL).
 *
 * ⟨D5⟩ — the origin lives in the ROUTE, not in a parameter: a song reached through a setlist is addressed
 * `/bands/:bandId/setlists/:setlistId/songs/:songId`, and Back is the path with the last two segments
 * removed. VLL asked for this shape and the repo already agreed with him — the chart editor has carried its
 * parent in the path since T105 (`/songs/:songId/chart/:fileId`) and derives its own Back from it.
 *
 * What that buys over the `?from=setlist:<id>` this replaces, and why the rework was worth it:
 *
 *   - Back is DERIVED, not resolved. No lookup, no pending state, and the label cannot disagree with the
 *     destination because there is only one answer and the router already holds it.
 *   - The hostile-input class is UNREPRESENTABLE rather than validated. A path segment cannot contain "/",
 *     so `../../bands/other` is not a setlist id — it is a different route, or none.
 *   - Nothing parses an id, so there is no silent contract with whatever mints them.
 *
 * The label stays generic ("Back to setlist", never the setlist's name): that ruling was about what a
 * reader may be shown, not about how the origin travels, and it survives the change untouched.
 */

/** The destination and the words on it, produced together — the label must not lie about where the arrow
 *  goes, and with the origin in the route they cannot come apart. */
export type BackTarget = { to: string; label: string };

/** The link a setlist row writes. The ONE place this shape is built (⟨D5⟩), so the route and the link
 *  cannot drift apart. */
export const songHrefFromSetlist = (bandId: string, setlistId: string, songId: string) =>
  `/bands/${bandId}/setlists/${setlistId}/songs/${songId}`;

/**
 * backTarget answers from the route alone. A `setlistId` is present only when the reader actually came
 * through a setlist, because that is a different route.
 *
 * ⟨D3⟩, as corrected: a setlist that has been deleted, or belongs to someone else, is NOT silently
 * retargeted to the band. The reader was there minutes ago; taking them back and letting the setlist page
 * say what happened is honest, where quietly landing them somewhere else hides that something they were
 * using is gone. The setlist page already owns its own not-found and its own authorisation — a back button
 * re-deciding that would be a second copy of a truth that will drift.
 */
export function backTarget(bandId: string | undefined, setlistId: string | undefined): BackTarget {
  if (!bandId) return { to: "/bands", label: "Back to bands" };
  if (setlistId) {
    return { to: `/bands/${bandId}/setlists/${setlistId}`, label: "Back to setlist" };
  }
  return { to: `/bands/${bandId}`, label: "Back to band" };
}
