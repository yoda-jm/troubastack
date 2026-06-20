/**
 * Thin TroubaCore REST client. Same-origin in prod, proxied in dev (/api →
 * :8080), so the HttpOnly `trouba_session` cookie rides along automatically —
 * every request uses credentials:'include'. No business logic here (the server
 * owns all policy, I6); this just decodes JSON and surfaces {error} as ApiError.
 */

export type User = {
  id: string;
  username: string;
  displayName: string;
  email?: string;
  createdAt: string;
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

export type Invite = {
  id: string;
  bandId: string;
  identifier: string;
  kind: "username" | "email" | "uuid";
  status: "pending" | "accepted" | "declined";
  createdAt: string;
};

export type Song = {
  id: string;
  bandId: string;
  title: string;
  artist?: string;
  key?: string;
  tempo?: number;
  tags?: string[];
  notes?: string;
  createdAt: string;
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
};

export type SongPatch = {
  title?: string;
  artist?: string;
  key?: string;
  tempo?: number;
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
  songTitle?: string;
  songArtist?: string;
};

export type SetlistItemPatch = {
  keyOverride?: string;
  tempoOverride?: number;
  notes?: string;
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
};

export type AnnotationObject = {
  uuid: string;
  layerId: string;
  type: "freehand" | "rect" | "ellipse" | "line" | "text" | "highlight";
  points: AnnotationPoint[];
  page: number;
  text: string;
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

type Method = "GET" | "POST" | "PATCH" | "DELETE";

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

  fileUrl: (fileId: string) => `/api/files/${fileId}`,

  // ---- annotations (view-only) ----
  getAnnotations: (bandId: string, songId: string) =>
    request<AnnotationDoc>("GET", `/api/bands/${bandId}/songs/${songId}/annotations`),

  importAnnotations: (bandId: string, songId: string, doc: AnnotationDoc) =>
    request<AnnotationDoc>(
      "POST",
      `/api/bands/${bandId}/songs/${songId}/annotations/import`,
      doc,
    ),

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

  leaveBand: (bandId: string) => request<void>("POST", `/api/bands/${bandId}/leave`),

  listBandInvites: (bandId: string) =>
    request<{ invites: Invite[] }>("GET", `/api/bands/${bandId}/invites`).then((r) => r.invites),

  revokeInvite: (bandId: string, inviteId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/invites/${inviteId}`),

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

  updateSetlist: (bandId: string, setlistId: string, patch: SetlistPatch) =>
    request<{ setlist: Setlist }>(
      "PATCH",
      `/api/bands/${bandId}/setlists/${setlistId}`,
      patch,
    ).then((r) => r.setlist),

  deleteSetlist: (bandId: string, setlistId: string) =>
    request<void>("DELETE", `/api/bands/${bandId}/setlists/${setlistId}`),

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

  // ---- invites ----
  listInvites: () => request<{ invites: Invite[] }>("GET", "/api/invites").then((r) => r.invites),

  acceptInvite: (inviteId: string) =>
    request<unknown>("POST", `/api/invites/${inviteId}/accept`),

  declineInvite: (inviteId: string) =>
    request<unknown>("POST", `/api/invites/${inviteId}/decline`),
};
