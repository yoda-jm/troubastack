// The editor routes are the ONE place the router and the shell agree on. App.tsx mounts these <Route>s and
// Shell.tsx derives the full-bleed / pinch-clamp decision from the SAME entries (isFullbleedPath), so a route
// is mounted and full-bleed from a single entry — there is no second list to keep in step.
//
// This is the T175→T184 bug made impossible by construction, not merely discouraged: T175 added the setlist
// editor route and a path regex in Shell did not learn about it, so the editor opened from a setlist kept the
// app navbar and its bars drifted off the viewport. Adding a fourth editor route now is one entry below that
// both surfaces read.
import { lazy } from "react";
import type { ReactElement } from "react";
import { matchPath } from "react-router-dom";

// T112: the annotation editor + its chart-editor route pull in pdf.js and the whole drawing canvas — ~half
// the bundle, and code nobody reaching /login needs. Lazy, so an editor route loads it only when visited.
const SongEditor = lazy(() => import("./pages/SongEditor").then((m) => ({ default: m.SongEditor })));
const ChartEditorPage = lazy(() =>
  import("./pages/ChartEditorPage").then((m) => ({ default: m.ChartEditorPage })),
);

export const SONG_EDITOR_PATH = "/bands/:bandId/songs/:songId";
// T175 ⟨D5⟩ — the SAME editor, addressed through the setlist the reader came from, so Back is the path minus
// its last two segments. The flat route stays permanently (the song's canonical address, what the band list
// links to, every existing bookmark). Two routes, one component, on purpose.
export const SONG_EDITOR_VIA_SETLIST_PATH = "/bands/:bandId/setlists/:setlistId/songs/:songId";
export const CHART_EDITOR_PATH = "/bands/:bandId/songs/:songId/chart/:fileId";

/** The editor route table: path + the element to mount. App mounts these; isFullbleedPath reads their paths.
 *  Full-bleed includes the chart editor (the old path regex matched it by prefix; kept, now explicit). */
export const EDITOR_ROUTES: { path: string; element: ReactElement }[] = [
  { path: SONG_EDITOR_PATH, element: <SongEditor /> },
  { path: SONG_EDITOR_VIA_SETLIST_PATH, element: <SongEditor /> },
  { path: CHART_EDITOR_PATH, element: <ChartEditorPage /> },
];

/** True iff `pathname` is one of the editor routes (exact match, so a band or setlist page is never
 *  full-bleed). Derived from EDITOR_ROUTES, the same entries App mounts. */
export function isFullbleedPath(pathname: string): boolean {
  return EDITOR_ROUTES.some((r) => matchPath(r.path, pathname) != null);
}
