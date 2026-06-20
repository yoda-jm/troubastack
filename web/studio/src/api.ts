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
  createdAt: string;
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

type Method = "GET" | "POST";

async function request<T>(method: Method, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "include",
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

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

  // ---- invites ----
  listInvites: () => request<{ invites: Invite[] }>("GET", "/api/invites").then((r) => r.invites),

  acceptInvite: (inviteId: string) =>
    request<unknown>("POST", `/api/invites/${inviteId}/accept`),

  declineInvite: (inviteId: string) =>
    request<unknown>("POST", `/api/invites/${inviteId}/decline`),
};
