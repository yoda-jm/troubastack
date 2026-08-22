/**
 * Thin TroubaCore REST client. Same-origin in prod, proxied in dev (/api →
 * :8080), so the HttpOnly `trouba_session` cookie rides along automatically —
 * every request uses credentials:'include'. No business logic here (the server
 * owns all policy, I6); this just decodes JSON and surfaces {error} as ApiError.
 */

export type AvatarKind = "man" | "woman" | "neutral" | "";

export type User = {
  id: string;
  username: string;
  displayName: string;
  email?: string;
  avatarKind?: AvatarKind;
  createdAt: string;
};

export type InviteLink = {
  id: string;
  bandId: string;
  token: string;
  url: string;
  role: "member" | "conductor";
  expiresAt?: string;
  maxUses: number;
  uses: number;
  createdAt: string;
  revoked: boolean;
  valid: boolean;
  reason?: string;
};

export type InviteLinkPreview = {
  band: { id: string; name: string };
  role: "member" | "conductor";
  valid: boolean;
  reason?: string;
};

export type Role = "admin" | "conductor" | "member";

export type Band = {
  id: string;
  name: string;
  ownerId: string;
  createdAt: string;
};

export type MemberView = {
  user: User;
  role: Role;
};

/** Result of importing a .tband archive (T62): the new band plus a member reconciliation
 * report. `matched` accounts already existed and were attached; `created` accounts were
 * minted with no password (the people need a reset link to sign in); `invited`/`skipped`
 * are the T63 dispositions, and the dropped-* counts are personal content dropped because
 * its owner was invited/skipped. */
export type ImportReport = {
  band: Band;
  matched: string[];
  created: string[];
  invited: string[];
  skipped: string[];
  songs: number;
  files: number;
  setlists: number;
  droppedLayers?: number;
  droppedObjects?: number;
  droppedCues?: number;
  droppedSelections?: number;
};

/** T63 disposition for a member missing on the target server. */
export type ImportDisposition = "create" | "invite" | "skip";

/** A manifest member classified against the target server (T63). An `existing` account
 * (other than the importer) is consent-required — it can only be invited or skipped, never
 * created. `isCaller` is the importer themselves (always the band admin, no choice). */
export type PreviewMember = {
  username: string;
  displayName: string;
  role: Role;
  existing: boolean;
  isCaller: boolean;
};

/** Preview of a .tband before import (T63): the classified members, plus counts. */
export type ImportPreview = {
  bandName: string;
  members: PreviewMember[];
  songs: number;
  files: number;
  setlists: number;
};

export type Invite = {
  id: string;
  bandId: string;
  identifier: string;
  kind: "username" | "email" | "uuid";
  status: "pending" | "accepted" | "declined";
  createdAt: string;
};

/** A personal cue on a song (T50): a stable icon id + an optional "#rrggbb" tint.
 *  An unknown icon id renders as the `note` fallback (see cue-glyphs.tsx).
 *  GENERATED from proto/troubastack/v1/bundle.proto `SongCue` (T09) — imported from
 *  api.gen.ts (every field optional: a mirror types PARSED JSON) and re-exported so
 *  the rest of the studio keeps importing it from "../api". */
import type { SongCue } from "./api.gen";
export type { SongCue };
// The wire object-type set, GENERATED from proto and re-exported by ink (T09).
import type { ObjectType } from "@troubastack/ink";

export type Song = {
  id: string;
  bandId: string;
  title: string;
  artist?: string;
  key?: string;
  tempo?: number;
  meter?: string; // T86: canonical "N/D" ("" = unset = 4/4)
  tags?: string[];
  notes?: string;
  createdAt: string;
  /** The CALLER's own personal cues for this song (T50), as served on listSongs
   *  rows. Absent/[] = none. Per-member — never another member's. */
  myCues?: SongCue[];
};

export type SongFile = {
  id: string;
  songId?: string;
  bandId?: string;
  filename: string;
  contentType: string;
  size: number;
  displayOrder: number;
  createdAt: string;
  // Generated text charts (T19): the server renders these from an editable source.
  generated?: boolean;
  revision?: number;
};

export type SongPatch = {
  title?: string;
  artist?: string;
  key?: string;
  tempo?: number;
  meter?: string;
  tags?: string[];
  notes?: string;
};

export type Setlist = {
  id: string;
  bandId: string;
  name: string;
  eventDate?: string;
  venue?: string;
  notes?: string;
  createdAt: string;
  // P201 rehearsal live mode: when set and in the future, edits to this setlist's
  // songs auto-bake. Self-expiring (server-side window). Absent/zero = off. Prefer the
  // server-computed `live` boolean from setSetlistLive over re-deriving from this.
  liveUntil?: string;
};

export type SetlistPatch = {
  name?: string;
  eventDate?: string;
  venue?: string;
  notes?: string;
};

export type SetlistItem = {
  id: string;
  setlistId?: string;
  songId: string;
  position: number;
  keyOverride?: string;
  tempoOverride?: number;
  notes?: string;
  // T23: a bench/on-call item — baked + jumpable on Stage, outside the running order.
  onCall?: boolean;
  songTitle?: string;
  songArtist?: string;
  // T60: burn the chart transposed to keyOverride at bake. songKey/hasChart are view
  // hints for the checkbox greying (the client parses the live-edited keyOverride).
  transposeChords?: boolean;
  songKey?: string;
  hasChart?: boolean;
};

export type SetlistItemPatch = {
  keyOverride?: string;
  tempoOverride?: number;
  notes?: string;
  onCall?: boolean;
  transposeChords?: boolean;
};

/** A baked concert (the proto AvailableConcert shape, B03) — 64-bit ints arrive as
 *  JSON strings (canonical). `downloadUrl`/`bakedBy` are server extras. */
export type Concert = {
  concertId: string;
  name: string;
  currentRev: string; // uint64 as string
  updatedAt: string; // int64 epoch seconds as string
  finalLocked?: boolean;
  songs: { songId: string; rev: string }[];
  bakedBy?: string;
  downloadUrl: string;
  // T60: per-song bake warnings (e.g. a transposed item that wasn't eligible at bake).
  // Present only on the bake POST response.
  warnings?: string[];
};

// ---- annotations (view-only) ----

export type AnnotationZone = "conductor" | "shared" | "personal";

export type AnnotationLayer = {
  id: string;
  fileId: string;
  name: string;
  ownerId: string;
  zone: AnnotationZone;
  order: number;
  access: "rw" | "ro";
  mandatory: boolean;
  roleTag: string;
};

export type AnnotationPoint = { x: number; y: number };

export type AnnotationStyle = {
  color: string;
  opacity: number;
  width: number;
  fontSize: number;
  // Unified shape model (rect/ellipse): paint interior / draw border / blend mode.
  // Absent flags mean "infer legacy default" in the renderer (back-compat).
  fill?: boolean;
  stroke?: boolean;
  blend?: "normal" | "multiply";
};

export type AnnotationObject = {
  uuid: string;
  layerId: string;
  // The wire object-type set, GENERATED from proto/troubastack/v1/object.proto (T09,
  // via ink's ObjectType). Includes "icon" (T51) — the old inline union had drifted.
  type: ObjectType;
  points: AnnotationPoint[];
  page: number;
  text: string;
  // Z-order WITHIN the object's layer (T27). Rendered ascending; ties break by
  // createdAt then uuid. Default 0. Changed via a `reorder` mutation.
  order: number;
  // Author-stamped creation time (unix ms); the z-order tiebreak after `order`.
  // Server-stamped on create; 0 for objects created before this field existed.
  createdAt: number;
  style: AnnotationStyle;
};

export type AnnotationDoc = {
  layers: AnnotationLayer[];
  objects: AnnotationObject[];
};

/** ApiError carries the HTTP status and the server's {error} message. */
export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

/** The caller's personal ordered file view: `files` is MY selection (or, when
 *  unset, all pool files in displayOrder) and `customized` flags whether I've
 *  saved an explicit selection. */
export type MyFiles = {
  files: SongFile[];
  customized: boolean;
};

/** OPS02: one downloadable native app binary the server carries. */
export type AppBinary = {
  platform: string;
  version: string;
  size: number;
  path: string; // download URL path (e.g. /apps/troubashare.apk)
  filename: string; // versioned download filename
};

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

async function request<T>(method: Method, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "include",
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  return decode<T>(res);
}

/** Multipart upload (FormData) — the browser sets the Content-Type boundary. */
async function upload<T>(path: string, form: FormData): Promise<T> {
  const res = await fetch(path, { method: "POST", credentials: "include", body: form });
  return decode<T>(res);
}

async function decode<T>(res: Response): Promise<T> {
  if (res.status === 204) {
    return undefined as T;
  }

  let payload: unknown = undefined;
  const text = await res.text();
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = undefined;
    }
  }

  if (!res.ok) {
    const msg =
      payload && typeof payload === "object" && "error" in payload
        ? String((payload as { error: unknown }).error)
        : `Request failed (${res.status})`;
    throw new ApiError(res.status, msg);
  }

  return payload as T;
}

export const api = {
  // ---- auth ----
  register: (input: { username: string; displayName: string; password: string; email?: string }) =>
    request<{ user: User }>("POST", "/api/auth/register", input).then((r) => r.user),

  login: (input: { username: string; password: string }) =>
    request<{ user: User }>("POST", "/api/auth/login", input).then((r) => r.user),

  logout: () => request<void>("POST", "/api/auth/logout"),

  me: () => request<{ user: User }>("GET", "/api/me").then((r) => r.user),

  updateProfile: (patch: { displayName?: string; email?: string; avatarKind?: AvatarKind }) =>
    request<{ user: User }>("PATCH", "/api/me", patch).then((r) => r.user),

  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>("POST", "/api/me/password", { currentPassword, newPassword }),

  // ---- bands ----
  listBands: () => request<{ bands: Band[] }>("GET", "/api/bands").then((r) => r.bands),

  createBand: (name: string) =>
    request<{ band: Band }>("POST", "/api/bands", { name }).then((r) => r.band),

  getBand: (bandId: string) =>
    request<{ band: Band; myRole: Role }>("GET", `/api/bands/${bandId}`),

  members: (bandId: string) =>
    request<{ members: MemberView[] }>("GET", `/api/bands/${bandId}/members`).then((r) => r.members),

  invite: (bandId: string, identifier: string, kind: Invite["kind"]) =>
    request<{ invite: Invite }>("POST", `/api/bands/${bandId}/invites`, { identifier, kind }).then(
      (r) => r.invite,
    ),

  // ---- whole-band export/import (T62) ----
  // Export the band as a portable .tband archive (admin-only, gated server-side). Returns
  // the zip Blob plus the server-suggested filename (from Content-Disposition).
  exportBand: async (bandId: string): Promise<{ blob: Blob; filename: string }> => {
    const res = await fetch(`/api/bands/${bandId}/export`, { credentials: "include" });
    if (!res.ok) {
      await decode<void>(res); // throws ApiError with the server message
    }
    const disp = res.headers.get("Content-Disposition") ?? "";
    const match = /filename="?([^"]+)"?/.exec(disp);
    return { blob: await res.blob(), filename: match?.[1] ?? `band-${bandId}.tband.zip` };
  },

  // Preview a .tband before importing (T63): classify members (matched vs missing) and
  // report counts, without writing anything.
  previewImport: (file: File) => {
    const form = new FormData();
    form.append("file", file);
    return upload<ImportPreview>("/api/bands/import:preview", form);
  },

  // Import a .tband archive → a NEW band owned by the caller. `dispositions` (T63) maps a
  // missing member's username to create|invite|skip; omitted members default to create.
  // Returns the reconciliation report.
  importBand: (file: File, dispositions?: Record<string, ImportDisposition>) => {
    const form = new FormData();
    form.append("file", file);
    if (dispositions && Object.keys(dispositions).length > 0) {
      form.append("dispositions", JSON.stringify(dispositions));
    }
    return upload<ImportReport>("/api/bands/import", form);
  },

  // ---- songs ----
  listSongs: (bandId: string) =>
    request<{ songs: Song[] }>("GET", `/api/bands/${bandId}/songs`).then((r) => r.songs),

  createSong: (bandId: string, title: string, artist?: string) =>
    request<{ song: Song }>("POST", `/api/bands/${bandId}/songs`, { title, artist }).then(
      (r) => r.song,
    ),

  getSong: (bandId: string, songId: string) =>
    request<{ songs: Song[] }>("GET", `/api/bands/${bandId}/songs`).then(
      (r) => r.songs.find((s) => s.id === songId) ?? null,
    ),

  updateSong: (bandId: string, songId: string, patch: SongPatch) =>
    request<{ song: Song }>("PATCH", `/api/bands/${bandId}/songs/${songId}`, patch).then(
      (r) => r.song,
    ),

  deleteSong: (bandId: string, songId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/songs/${songId}`),

  // ---- song files ----
  listFiles: (bandId: string, songId: string) =>
    request<{ files: SongFile[] }>("GET", `/api/bands/${bandId}/songs/${songId}/files`).then(
      (r) => r.files,
    ),

  // ---- text charts (T19): write a chart in the tiny dialect; the server renders
  // it to a PDF in the pool. Edit re-renders in place (same file id, revision++);
  // saveChartSource sends the base revision for LWW conflict detection (409).
  createTextChart: (bandId: string, songId: string, source: string) =>
    request<{ file: SongFile }>("POST", `/api/bands/${bandId}/songs/${songId}/text-charts`, {
      source,
    }).then((r) => r.file),

  // T37: best-effort server-side lyrics fetch (azlyrics/any URL), SSRF-guarded. Returns
  // the normalized text on success; a "blocked"/"error" status is a NORMAL outcome (the
  // UI falls back to paste), never thrown — azlyrics is Cloudflare-gated and honest GETs
  // often bounce.
  lyricsImport: (bandId: string, url: string) =>
    request<{ status: "ok" | "blocked" | "error"; text?: string; reason?: string }>(
      "POST",
      `/api/bands/${bandId}/lyrics-import`,
      { url },
    ),

  // Render a chart to PDF bytes WITHOUT persisting (T25 preview). Returns the PDF
  // Blob; throws ApiError with the server's message (e.g. bad chars) on failure.
  previewTextChart: async (bandId: string, songId: string, source: string): Promise<Blob> => {
    const res = await fetch(`/api/bands/${bandId}/songs/${songId}/text-charts:preview`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ source }),
    });
    if (!res.ok) {
      let msg = `Request failed (${res.status})`;
      try {
        const j = (await res.json()) as { error?: unknown };
        if (j && j.error != null) msg = String(j.error);
      } catch {
        // non-JSON error body — keep the generic message
      }
      throw new ApiError(res.status, msg);
    }
    return res.blob();
  },

  getChartSource: (bandId: string, songId: string, fileId: string) =>
    request<{ file: SongFile; source: string }>(
      "GET",
      `/api/bands/${bandId}/songs/${songId}/files/${fileId}/chart-source`,
    ),

  saveChartSource: (
    bandId: string,
    songId: string,
    fileId: string,
    baseRevision: number,
    source: string,
  ) =>
    request<{ file: SongFile }>(
      "PUT",
      `/api/bands/${bandId}/songs/${songId}/files/${fileId}/chart-source`,
      { source, baseRevision },
    ).then((r) => r.file),

  // transposeChartSource (T60 surface 1): transpose a generated chart's source by a
  // target key (or a raw semitone count when the song key isn't parseable). dryRun
  // returns the transposed source for preview without persisting; otherwise it saves
  // in place (LWW on baseRevision → 409) and, with updateSongKey, sets the song key.
  transposeChartSource: (
    bandId: string,
    songId: string,
    fileId: string,
    opts: {
      targetKey?: string;
      semitones?: number;
      updateSongKey?: boolean;
      baseRevision: number;
      dryRun: boolean;
    },
  ) =>
    request<{ file?: SongFile; source: string }>(
      "POST",
      `/api/bands/${bandId}/songs/${songId}/files/${fileId}/chart-source:transpose`,
      opts,
    ),

  uploadFile: (bandId: string, songId: string, file: File) => {
    const form = new FormData();
    form.append("file", file);
    return upload<{ file: SongFile }>(`/api/bands/${bandId}/songs/${songId}/files`, form).then(
      (r) => r.file,
    );
  },

  updateFile: (
    bandId: string,
    songId: string,
    fileId: string,
    patch: { filename?: string; displayOrder?: number },
  ) =>
    request<{ file: SongFile }>(
      "PATCH",
      `/api/bands/${bandId}/songs/${songId}/files/${fileId}`,
      patch,
    ).then((r) => r.file),

  deleteFile: (bandId: string, songId: string, fileId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/songs/${songId}/files/${fileId}`),

  // T67 — a re-rendered chart keeps the same file id but bumps `revision`. Pinning the
  // revision in the URL makes "new render = new URL", so the browser can't serve the stale
  // cached bytes and the viewer's fetch effect re-runs. Pass the SongFile's revision wherever
  // one is in hand; the bare form (no revision) is only for callers without a SongFile.
  fileUrl: (fileId: string, revision?: number) =>
    revision != null ? `/api/files/${fileId}?rev=${revision}` : `/api/files/${fileId}`,

  // ---- per-member file selection ("my files") ----
  // The pool stays shared (listFiles); these endpoints are the caller's own
  // ordered view over it. Default (unset) returns all pool files, customized=false.
  getMyFiles: (bandId: string, songId: string) =>
    request<MyFiles>("GET", `/api/bands/${bandId}/songs/${songId}/my-files`),

  setMyFiles: (bandId: string, songId: string, fileIds: string[]) =>
    request<MyFiles>("PUT", `/api/bands/${bandId}/songs/${songId}/my-files`, { fileIds }),

  clearMyFiles: (bandId: string, songId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/songs/${songId}/my-files`),

  // ---- per-member song cues ("my cues", T50) ----
  // Self-only by construction (keyed to the caller). GET returns [] when unset;
  // PUT replaces the list (an empty list clears it).
  getMyCues: (bandId: string, songId: string) =>
    request<{ cues: SongCue[] }>("GET", `/api/bands/${bandId}/songs/${songId}/my-cues`).then(
      (r) => r.cues,
    ),

  setMyCues: (bandId: string, songId: string, cues: SongCue[]) =>
    request<{ cues: SongCue[] }>("PUT", `/api/bands/${bandId}/songs/${songId}/my-cues`, {
      cues,
    }).then((r) => r.cues),

  // ---- annotations (view-only) ----
  getAnnotations: (bandId: string, songId: string) =>
    request<AnnotationDoc>("GET", `/api/bands/${bandId}/songs/${songId}/annotations`),

  importAnnotations: (bandId: string, songId: string, doc: AnnotationDoc) =>
    request<AnnotationDoc>(
      "POST",
      `/api/bands/${bandId}/songs/${songId}/annotations/import`,
      doc,
    ),

  // P201: the live-mode setlists (id + name) that contain this song right now — the
  // editor's LIVE-banner signal. Empty = not live.
  liveSetlistsForSong: (bandId: string, songId: string) =>
    request<{ setlists: { id: string; name: string }[] }>(
      "GET",
      `/api/bands/${bandId}/songs/${songId}/live-setlists`,
    ).then((r) => r.setlists),

  // ---- bands (admin) ----
  updateBand: (bandId: string, name: string) =>
    request<{ band: Band }>("PATCH", `/api/bands/${bandId}`, { name }).then((r) => r.band),

  deleteBand: (bandId: string) => request<void>("DELETE", `/api/bands/${bandId}`),

  updateMemberRole: (bandId: string, userId: string, role: Role) =>
    request<{ member: MemberView }>("PATCH", `/api/bands/${bandId}/members/${userId}`, {
      role,
    }).then((r) => r.member),

  removeMember: (bandId: string, userId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/members/${userId}`),

  // ---- password reset (T21) ----
  // Admin mints a one-time reset link for a member; the server returns a relative
  // path (it doesn't know its public origin) — the caller joins it with origin.
  issuePasswordReset: (bandId: string, userId: string) =>
    request<{ token: string; resetPath: string }>(
      "POST",
      `/api/bands/${bandId}/members/${userId}/password-reset`,
    ),

  // Public (the token IS the credential): who the reset is for, then set it.
  previewPasswordReset: (token: string) =>
    request<{ user: User }>("GET", `/api/password-reset/${token}`).then((r) => r.user),

  submitPasswordReset: (token: string, newPassword: string) =>
    request<void>("POST", `/api/password-reset/${token}`, { newPassword }),

  leaveBand: (bandId: string) => request<void>("POST", `/api/bands/${bandId}/leave`),

  listBandInvites: (bandId: string) =>
    request<{ invites: Invite[] }>("GET", `/api/bands/${bandId}/invites`).then((r) => r.invites),

  revokeInvite: (bandId: string, inviteId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/invites/${inviteId}`),

  // ---- band invite links (tokenized join links; admin) ----
  listInviteLinks: (bandId: string) =>
    request<{ links: InviteLink[] }>("GET", `/api/bands/${bandId}/invite-links`).then(
      (r) => r.links,
    ),

  createInviteLink: (
    bandId: string,
    input: { role?: "member" | "conductor"; expiresInHours?: number; maxUses?: number },
  ) => request<InviteLink>("POST", `/api/bands/${bandId}/invite-links`, input),

  revokeInviteLink: (bandId: string, id: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/invite-links/${id}`),

  // ---- join links (any authenticated user) ----
  previewInviteLink: (token: string) =>
    request<InviteLinkPreview>("GET", `/api/invite-links/${token}`),

  acceptInviteLink: (token: string) =>
    request<{ band: Band }>("POST", `/api/invite-links/${token}/accept`).then((r) => r.band),

  // ---- setlists ----
  listSetlists: (bandId: string) =>
    request<{ setlists: Setlist[] }>("GET", `/api/bands/${bandId}/setlists`).then(
      (r) => r.setlists,
    ),

  createSetlist: (bandId: string, input: { name: string } & SetlistPatch) =>
    request<{ setlist: Setlist }>("POST", `/api/bands/${bandId}/setlists`, input).then(
      (r) => r.setlist,
    ),

  getSetlist: (bandId: string, setlistId: string) =>
    request<{ setlist: Setlist; items: SetlistItem[] }>(
      "GET",
      `/api/bands/${bandId}/setlists/${setlistId}`,
    ),

  // Deep-copy a setlist (member-level); returns the new setlist (T20).
  duplicateSetlist: (bandId: string, setlistId: string) =>
    request<{ setlist: Setlist }>(
      "POST",
      `/api/bands/${bandId}/setlists/${setlistId}/duplicate`,
    ).then((r) => r.setlist),

  updateSetlist: (bandId: string, setlistId: string, patch: SetlistPatch) =>
    request<{ setlist: Setlist }>(
      "PATCH",
      `/api/bands/${bandId}/setlists/${setlistId}`,
      patch,
    ).then((r) => r.setlist),

  deleteSetlist: (bandId: string, setlistId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/setlists/${setlistId}`),

  // P201: toggle rehearsal live mode (admin-only). Returns the setlist + the
  // server-computed `live` boolean (as of the server clock).
  setSetlistLive: (bandId: string, setlistId: string, live: boolean) =>
    request<{ setlist: Setlist; live: boolean }>(
      "POST",
      `/api/bands/${bandId}/setlists/${setlistId}/live`,
      { live },
    ),

  addSetlistItem: (bandId: string, setlistId: string, songId: string) =>
    request<{ item: SetlistItem }>(
      "POST",
      `/api/bands/${bandId}/setlists/${setlistId}/items`,
      { songId },
    ).then((r) => r.item),

  updateSetlistItem: (
    bandId: string,
    setlistId: string,
    itemId: string,
    patch: SetlistItemPatch,
  ) =>
    request<{ item: SetlistItem }>(
      "PATCH",
      `/api/bands/${bandId}/setlists/${setlistId}/items/${itemId}`,
      patch,
    ).then((r) => r.item),

  removeSetlistItem: (bandId: string, setlistId: string, itemId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/setlists/${setlistId}/items/${itemId}`),

  reorderSetlist: (bandId: string, setlistId: string, orderedItemIds: string[]) =>
    request<{ items: SetlistItem[] }>(
      "POST",
      `/api/bands/${bandId}/setlists/${setlistId}/reorder`,
      { orderedItemIds },
    ).then((r) => r.items),

  // ---- bake / concerts (B02) ----
  // A setlist bakes to a "concert" (concertId === setlistId); each bake bumps
  // concertRev. The band-wide bake is admin-only and is THE bake (P205); the personal
  // "?scope=mine" variant (B07) was retired. Listing/download stay member-scoped to
  // the band (old `${setlistId}~${userId}` variant concerts remain viewable).
  // P205: the optional layerDefaults map (layer name → default-on) is the bake
  // dialog's explicit capture — the server stamps LayerImage.default_on from it
  // (absent ⇒ legacy compute).
  bakeSetlist: (bandId: string, setlistId: string, layerDefaults?: Record<string, boolean>) =>
    request<Concert>(
      "POST",
      `/api/bands/${bandId}/setlists/${setlistId}/bake`,
      layerDefaults ? { layerDefaults } : undefined,
    ),
  listConcerts: (bandId: string) =>
    request<{ concerts: Concert[] }>("GET", `/api/bands/${bandId}/concerts`).then(
      (r) => r.concerts,
    ),

  // OPS02: the native app binaries this server carries (empty when the image was
  // built without them / in dev). The band page shows a "Get the app" card from this.
  listApps: () => request<{ apps: AppBinary[] }>("GET", "/api/apps").then((r) => r.apps),

  // T57: the printable-PDF URL for a concert (paper fallback). Derived from the
  // concert's bundle downloadUrl (…/bundle → …/pdf) — same auth gating; the server
  // composites the caller's view (mandatory + untagged shared + the caller's own
  // personal layers) via the shared P205 view-resolution rule, so print == screen.
  concertPdfUrl: (c: Concert) => c.downloadUrl.replace(/\/bundle$/, "/pdf"),

  // ---- invites ----
  listInvites: () => request<{ invites: Invite[] }>("GET", "/api/invites").then((r) => r.invites),

  acceptInvite: (inviteId: string) =>
    request<unknown>("POST", `/api/invites/${inviteId}/accept`),

  declineInvite: (inviteId: string) =>
    request<unknown>("POST", `/api/invites/${inviteId}/decline`),
};
