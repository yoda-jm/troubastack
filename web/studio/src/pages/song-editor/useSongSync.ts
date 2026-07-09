/**
 * useSongSync (T15 part of the Viewer split) — owns the realtime spine of the
 * editor: the per-song WebSocket lifecycle and the live annotation document it
 * drives. Extracted VERBATIM from Viewer.tsx; behavior unchanged.
 *
 * `doc` ({layers, objects}) is the shared state the whole editor reads: the sync
 * client's onState replaces it, the PDF/overlay paint reads it, and the editing
 * handlers mutate it optimistically (via syncRef). Visibility defaults are merged
 * here too so a layer arriving over the wire (e.g. my new personal layer) shows
 * immediately. onReject surfaces a brief notice (the optimistic rollback itself
 * already happened in the SyncClient). The load-once REST seed stays in Viewer and
 * writes through setDoc/setVisible returned here.
 */
import { useEffect, useRef, useState } from "react";
import type { AnnotationDoc, AnnotationLayer } from "../../api";
import { SyncClient, type SyncState } from "../../sync";
import type { LayerVisibility } from "./helpers";

/** Default per-viewer visibility (I-style policy): mandatory + shared + my own
 *  personal layers ON; other members' (non-shared, non-mandatory) OFF. */
export function defaultVisibility(
  layers: AnnotationLayer[],
  myUserId: string | null,
): LayerVisibility {
  const vis: LayerVisibility = {};
  for (const l of layers) {
    if (l.mandatory) vis[l.id] = true;
    else if (l.zone === "shared") vis[l.id] = true;
    else if (l.zone === "personal" && myUserId != null && l.ownerId === myUserId) vis[l.id] = true;
    else if (l.zone === "conductor") vis[l.id] = true;
    else vis[l.id] = false;
  }
  return vis;
}

export type ConnStatus = "connecting" | "open" | "closed";

export function useSongSync(bandId: string, songId: string, myUserId: string | null) {
  const [doc, setDoc] = useState<AnnotationDoc>({ layers: [], objects: [] });
  const [visible, setVisible] = useState<LayerVisibility>({});
  const [connStatus, setConnStatus] = useState<ConnStatus>("connecting");
  // A brief inline notice when the server rejects one of our writes.
  const [rejectNotice, setRejectNotice] = useState<string | null>(null);
  // The realtime client for this song; null until a connection is opened.
  const syncRef = useRef<SyncClient | null>(null);

  // ---- realtime sync: one WebSocket per open song ----
  // The live document is driven by the WS (snapshot + echoes). The REST GET in the
  // Viewer seeds the first paint; once the snapshot lands it becomes authoritative.
  // New layers arriving over the wire default to visible (defaultVisibility).
  useEffect(() => {
    const client = new SyncClient(bandId, songId, {
      onState: (s: SyncState) => {
        setDoc({ layers: s.layers, objects: s.objects });
        // Ensure any layer we don't yet have a visibility entry for gets a sane
        // default (so my new personal layer shows immediately, etc.).
        setVisible((prev) => {
          let changed = false;
          const next = { ...prev };
          const defaults = defaultVisibility(s.layers, myUserId);
          for (const l of s.layers) {
            if (!(l.id in next)) {
              next[l.id] = defaults[l.id];
              changed = true;
            }
          }
          return changed ? next : prev;
        });
      },
      onStatus: setConnStatus,
      onReject: (_uuid, reason) => {
        // The editable-layer model should prevent forbidden writes, but if the
        // server still rejects (e.g. a stale layer), roll back is already done —
        // show a brief inline notice so the user knows the edit didn't stick.
        setRejectNotice(
          reason === "forbidden"
            ? "That layer is read-only — your edit wasn't saved."
            : reason === "deleted-remotely"
              ? "That object was deleted by someone else."
              : "Your edit couldn't be saved (out of date).",
        );
        window.setTimeout(() => setRejectNotice(null), 4000);
      },
    });
    syncRef.current = client;
    client.connect();
    return () => {
      client.close();
      syncRef.current = null;
    };
  }, [bandId, songId, myUserId]);

  return { doc, setDoc, visible, setVisible, connStatus, rejectNotice, syncRef };
}
