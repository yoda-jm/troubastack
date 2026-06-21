/**
 * Realtime sync client for the annotation editor.
 *
 * Owns ONE WebSocket per open song and turns the server's frame protocol
 * (snapshot | echo | reject) into a live, in-memory annotation document the
 * editor renders from. The wire format mirrors the Go core EXACTLY
 * (core/internal/sync/{conn,apply,mapping}.go):
 *
 *   server → client:
 *     {type:"snapshot", layers:[Layer], objects:[Object], seq}
 *     {type:"echo", mutation:<Mutation>}        // broadcast incl. sender
 *     {type:"reject", uuid, reason}             // to the sender only
 *   client → server:
 *     {type:"mutation", mutation:<Mutation>}
 *
 *   Mutation = {kind, uuid, object?, layer?, baseVersion?, clientTs?, summary?}
 *   kind ∈ create|move|resize|setStyle|setText|delete|restore|layerCreate
 *
 * The server derives version/authorId/seq; the client never sends them.
 *
 * Reconciliation is by uuid: create/move/resize/setStyle/setText/restore upsert
 * the object; delete removes it; layerCreate adds the layer. Optimistic local
 * ops are tracked by uuid so a `reject` can roll them back to the pre-op state.
 */
import type { AnnotationLayer, AnnotationObject } from "./api";

export type MutationKind =
  | "create"
  | "move"
  | "resize"
  | "setStyle"
  | "setText"
  | "delete"
  | "restore"
  | "layerCreate"
  | "layerUpdate";

/** The wire mutation. version/authorId/seq are server-derived; never sent. */
export type Mutation = {
  kind: MutationKind;
  uuid: string;
  object?: AnnotationObject;
  layer?: AnnotationLayer;
  baseVersion?: number;
  clientTs?: number;
  summary?: string;
  // present only on echoes (server-authoritative):
  seq?: number;
  authorId?: string;
};

type SnapshotFrame = {
  type: "snapshot";
  layers: AnnotationLayer[];
  objects: AnnotationObject[];
  seq: number;
};
type EchoFrame = { type: "echo"; mutation: Mutation };
export type RejectReason = "deleted-remotely" | "stale" | "forbidden";
type RejectFrame = {
  type: "reject";
  uuid: string;
  reason: RejectReason;
};
type ServerFrame = SnapshotFrame | EchoFrame | RejectFrame;

/** The live document the editor renders from. */
export type SyncState = {
  layers: AnnotationLayer[];
  objects: AnnotationObject[];
};

export type SyncEvents = {
  /** Called whenever the live state changes (snapshot, echo, reject rollback). */
  onState: (state: SyncState) => void;
  /** Connection status, for a UI dot / reconnect note. */
  onStatus?: (status: "connecting" | "open" | "closed") => void;
  /** Called when the server rejects one of OUR optimistic ops (after rollback),
   *  so the UI can surface a brief inline notice (e.g. a forbidden write). */
  onReject?: (uuid: string, reason: RejectReason) => void;
};

/** Snapshot of one optimistic op, kept until its echo lands (so a reject can
 *  restore the prior object — or remove it for an optimistic create). */
type Pending = {
  uuid: string;
  /** The object that existed BEFORE the optimistic op (undefined if it was a create). */
  prev?: AnnotationObject;
};

function wsUrl(bandId: string, songId: string): string {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${location.host}/api/bands/${bandId}/songs/${songId}/ws`;
}

/**
 * SyncClient manages the socket lifecycle and the live document. The editor:
 *   - reads `state` (or subscribes via onState) to render,
 *   - calls send* helpers to emit mutations (optimistically updating local state),
 *   - calls close() on unmount / song switch.
 */
export class SyncClient {
  private ws: WebSocket | null = null;
  private closedByUs = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectDelay = 500;

  private layers = new Map<string, AnnotationLayer>();
  private objects = new Map<string, AnnotationObject>();
  /** Optimistic ops awaiting their echo, keyed by uuid (last write wins). */
  private pending = new Map<string, Pending>();

  constructor(
    private bandId: string,
    private songId: string,
    private events: SyncEvents,
  ) {}

  connect(): void {
    this.closedByUs = false;
    this.open();
  }

  private open(): void {
    this.events.onStatus?.("connecting");
    let ws: WebSocket;
    try {
      ws = new WebSocket(wsUrl(this.bandId, this.songId));
    } catch {
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;
    ws.onopen = () => {
      this.reconnectDelay = 500;
      this.events.onStatus?.("open");
    };
    ws.onmessage = (ev) => this.onFrame(ev.data);
    ws.onclose = () => {
      this.events.onStatus?.("closed");
      if (!this.closedByUs) this.scheduleReconnect();
    };
    ws.onerror = () => {
      // onclose follows; reconnect is handled there.
    };
  }

  private scheduleReconnect(): void {
    if (this.closedByUs || this.reconnectTimer) return;
    const delay = this.reconnectDelay;
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, 5000);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.open();
    }, delay);
  }

  close(): void {
    this.closedByUs = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
  }

  /** Current live state (defensive copies for React state identity). */
  get state(): SyncState {
    return {
      layers: [...this.layers.values()],
      objects: [...this.objects.values()],
    };
  }

  private emit(): void {
    this.events.onState(this.state);
  }

  // ---- incoming frames --------------------------------------------------

  private onFrame(data: unknown): void {
    if (typeof data !== "string") return;
    let frame: ServerFrame;
    try {
      frame = JSON.parse(data) as ServerFrame;
    } catch {
      return;
    }
    switch (frame.type) {
      case "snapshot":
        this.applySnapshot(frame);
        break;
      case "echo":
        this.applyEcho(frame.mutation);
        break;
      case "reject":
        this.applyReject(frame);
        break;
    }
  }

  private applySnapshot(snap: SnapshotFrame): void {
    // A snapshot is the authoritative HEAD — reseed from scratch. Reconnect
    // re-snapshots, so this also recovers any echoes missed while offline.
    this.layers = new Map((snap.layers ?? []).map((l) => [l.id, l]));
    this.objects = new Map((snap.objects ?? []).map((o) => [o.uuid, o]));
    this.pending.clear();
    this.emit();
  }

  private applyEcho(m: Mutation): void {
    // The echo is authoritative; its arrival reconciles any matching optimistic op.
    if (m.kind === "layerCreate" || m.kind === "layerUpdate") {
      if (m.layer) this.layers.set(m.layer.id, m.layer);
      this.emit();
      return;
    }
    const uuid = m.uuid || m.object?.uuid || "";
    if (!uuid) return;
    this.pending.delete(uuid);

    switch (m.kind) {
      case "delete":
        this.objects.delete(uuid);
        break;
      case "create":
      case "move":
      case "resize":
      case "setStyle":
      case "setText":
      case "restore":
        if (m.object) this.objects.set(uuid, m.object);
        break;
    }
    this.emit();
  }

  private applyReject(r: RejectFrame): void {
    const p = this.pending.get(r.uuid);
    this.pending.delete(r.uuid);
    if (!p) return; // not ours / already reconciled
    // Roll back to the pre-op object, or remove it if the op was a create.
    if (p.prev) this.objects.set(r.uuid, p.prev);
    else this.objects.delete(r.uuid);
    this.emit();
    this.events.onReject?.(r.uuid, r.reason);
  }

  // ---- outgoing mutations ----------------------------------------------

  private sendMutation(m: Mutation): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: "mutation", mutation: m }));
    }
    // If the socket is down the optimistic state still shows; the next
    // snapshot on reconnect reconciles (the local create simply won't persist).
  }

  /** Optimistically upsert `obj` and send a create. */
  createObject(obj: AnnotationObject): void {
    const prev = this.objects.get(obj.uuid);
    this.pending.set(obj.uuid, { uuid: obj.uuid, prev });
    this.objects.set(obj.uuid, obj);
    this.emit();
    this.sendMutation({ kind: "create", uuid: obj.uuid, object: obj, clientTs: Date.now() });
  }

  /** Optimistically replace `obj` and send `kind` (move/resize/setStyle/setText). */
  updateObject(kind: Exclude<MutationKind, "create" | "delete" | "layerCreate">, obj: AnnotationObject): void {
    const prev = this.objects.get(obj.uuid);
    this.pending.set(obj.uuid, { uuid: obj.uuid, prev });
    this.objects.set(obj.uuid, obj);
    this.emit();
    this.sendMutation({ kind, uuid: obj.uuid, object: obj, clientTs: Date.now() });
  }

  /** Optimistically remove and send a delete (server expects only the uuid). */
  deleteObject(uuid: string): void {
    const prev = this.objects.get(uuid);
    if (!prev) return;
    this.pending.set(uuid, { uuid, prev });
    this.objects.delete(uuid);
    this.emit();
    this.sendMutation({ kind: "delete", uuid, clientTs: Date.now() });
  }

  /** Optimistically add a layer and send a layerCreate. */
  createLayer(layer: AnnotationLayer): void {
    this.layers.set(layer.id, layer);
    this.emit();
    this.sendMutation({ kind: "layerCreate", uuid: "", layer, clientTs: Date.now() });
  }

  /** Optimistically replace a layer and send a layerUpdate (e.g. the lock/unlock
   *  access toggle, #4). A server reject reconciles on the next snapshot (layer ops
   *  aren't in the per-object pending/rollback set). */
  updateLayer(layer: AnnotationLayer): void {
    this.layers.set(layer.id, layer);
    this.emit();
    this.sendMutation({ kind: "layerUpdate", uuid: "", layer, clientTs: Date.now() });
  }
}
