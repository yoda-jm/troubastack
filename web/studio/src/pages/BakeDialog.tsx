/**
 * P205 Stage 1 — bake dialog. Baking captures which layers are ON by default in the
 * bundle (view-time default). Fable's ruling: NEVER capture silently — show it
 * explicitly ("Baking with: Cues ✓ · Form ✓ · My notes ✗ — edit?"), WYSIWYG seeded
 * from the layers' current default-visibility, editable, remembered per setlist. On
 * confirm the choice rides to the baker as `layerDefaults` (name → default-on).
 */
import { useEffect, useMemo, useRef, useState } from "react";
import { ApiError, api, type BakeProgress, type Concert } from "../api";
import { newUuid } from "../editor";
import { AudienceTag, audienceForZone } from "../components/AudienceTag";

type LayerRow = { name: string; zone: string; mandatory: boolean };

function rememberKey(setlistId: string) {
  return `trouba_bake_defaults_${setlistId}`;
}

export function BakeDialog({
  bandId,
  setlistId,
  songIds,
  onBake,
  onDone,
  onCancel,
}: {
  bandId: string;
  setlistId: string;
  songIds: string[];
  // onBake runs the actual bake POST (carrying the supplied id) and resolves with the
  // concert; the dialog owns the progress polling around it, so its lifecycle — and every
  // interval — is bounded by the dialog's own mount. onDone hands the concert to the parent.
  onBake: (layerDefaults: Record<string, boolean>, bakeId: string) => Promise<Concert>;
  onDone: (concert: Concert) => void;
  onCancel: () => void;
}) {
  const [layers, setLayers] = useState<LayerRow[] | null>(null);
  const [on, setOn] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false);
  const [bakeId, setBakeId] = useState<string | null>(null);
  const [progress, setProgress] = useState<BakeProgress | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Collect the distinct layers (by name) across the setlist's songs, then seed the
  // checkboxes: mandatory → always on; others → the remembered choice, else on.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const byName = new Map<string, LayerRow>();
      await Promise.all(
        songIds.map(async (songId) => {
          try {
            const doc = await api.getAnnotations(bandId, songId);
            for (const l of doc.layers) {
              const prev = byName.get(l.name);
              byName.set(l.name, {
                name: l.name,
                zone: l.zone,
                mandatory: (prev?.mandatory ?? false) || l.mandatory,
              });
            }
          } catch {
            /* a song with no annotations just contributes no layers */
          }
        }),
      );
      if (cancelled) return;
      const rows = [...byName.values()].sort((a, b) => a.name.localeCompare(b.name));
      let remembered: Record<string, boolean> = {};
      try {
        remembered = JSON.parse(sessionStorage.getItem(rememberKey(setlistId)) || "{}");
      } catch {
        /* ignore */
      }
      const seed: Record<string, boolean> = {};
      for (const r of rows) seed[r.name] = r.mandatory ? true : (remembered[r.name] ?? true);
      setLayers(rows);
      setOn(seed);
    })();
    return () => {
      cancelled = true;
    };
  }, [bandId, setlistId, songIds]);

  const title = "Bake setlist";
  const summary = useMemo(
    () => (layers ?? []).map((l) => `${l.name} ${on[l.name] ? "✓" : "✗"}`).join(" · "),
    [layers, on],
  );

  function toggle(name: string) {
    setOn((s) => ({ ...s, [name]: !s[name] }));
  }

  // Poll progress WHILE the bake POST is in flight. Tied to the dialog's mount: when busy
  // flips false (settled) or the dialog unmounts, the effect cleanup clears the interval —
  // no timer outlives the dialog (T99 §5). Overlapping ticks are skipped via inFlight.
  const inFlight = useRef(false);
  useEffect(() => {
    if (!busy || !bakeId) return;
    let stopped = false;
    const tick = async () => {
      if (inFlight.current) return; // don't queue behind a slow request
      inFlight.current = true;
      try {
        const p = await api.bakeProgress(bandId, setlistId, bakeId);
        if (!stopped && p) setProgress(p); // null ⇒ 404/error: keep the last line, degrade to "Baking…"
      } finally {
        inFlight.current = false;
      }
    };
    void tick();
    const timer = window.setInterval(() => void tick(), 1000);
    return () => {
      stopped = true;
      window.clearInterval(timer);
    };
  }, [busy, bakeId, bandId, setlistId]);

  async function confirm() {
    // newUuid, NOT crypto.randomUUID: the latter is secure-context-only and is `undefined`
    // on a plain-http:// LAN origin (how every band member not at the server box connects),
    // where it would throw here — before the POST — and the bake would silently never fire
    // (T32's class of bug; newUuid falls back to getRandomValues/Math.random).
    const id = newUuid();
    try {
      sessionStorage.setItem(rememberKey(setlistId), JSON.stringify(on));
    } catch {
      /* ignore */
    }
    setError(null);
    setProgress(null);
    setBakeId(id);
    setBusy(true);
    try {
      const concert = await onBake(on, id);
      onDone(concert); // parent reloads + unmounts us; the effect cleanup stops polling
    } catch (err) {
      setBusy(false); // stops polling; the dialog stays open so the failure is visible + retryable
      // Name the failing song from the server's terminal progress (the POST error carries the
      // song id; progress carries the human title), falling back to the POST error message.
      const finalP = await api.bakeProgress(bandId, setlistId, id).catch(() => null);
      if (finalP?.state === "failed" && finalP.song) {
        setError(`Couldn't bake “${finalP.song}”.${finalP.error ? ` (${finalP.error})` : ""}`);
      } else {
        setError(err instanceof ApiError ? err.message : "Bake failed");
      }
    }
  }

  // The one line the user watches. "Finishing…" for the T98 tail (done == total, no song)
  // instead of a frozen-looking "N of N"; "Baking…" for the pre-first-song window or when
  // progress is unavailable (degrade to today).
  function progressLabel(p: BakeProgress | null): string {
    if (p && p.state === "running") {
      if (p.song) return `Baking — song ${p.done} of ${p.total}: ${p.song}`;
      if (p.done > 0 && p.done === p.total) return "Finishing…";
    }
    return "Baking…";
  }

  return (
    <div className="bake-dialog-backdrop" data-testid="bake-dialog" role="dialog" aria-modal="true" aria-label={title}>
      <div className="bake-dialog card">
        <div className="card-head">
          <h2>{title}</h2>
          <AudienceTag audience="band" />
        </div>
        <p className="muted">
          Baking captures which layers show <em>by default</em> when the concert opens. Toggle any off
          you don’t want on at first — mandatory layers always show.
        </p>
        {layers == null ? (
          <p className="muted">Loading layers…</p>
        ) : layers.length === 0 ? (
          <p className="muted" data-testid="bake-dialog-nolayers">
            This concert has no annotation layers to configure.
          </p>
        ) : (
          <>
            <p className="muted bake-dialog-summary" data-testid="bake-dialog-summary">
              Baking with: {summary}
            </p>
            <ul className="list bake-dialog-list">
              {layers.map((l) => (
                <li key={l.name} className="bake-dialog-row" data-testid="bake-dialog-layer" data-layer={l.name}>
                  <label>
                    <input
                      type="checkbox"
                      data-testid="bake-dialog-toggle"
                      checked={!!on[l.name]}
                      disabled={l.mandatory || busy}
                      onChange={() => toggle(l.name)}
                    />
                    <span className="bake-dialog-name">{l.name}</span>
                  </label>
                  <AudienceTag
                    audience={audienceForZone(l.zone)}
                    note={l.zone === "conductor" ? "conductor" : l.mandatory ? "required" : undefined}
                  />
                </li>
              ))}
            </ul>
          </>
        )}
        {busy && (
          <p className="muted bake-dialog-progress" data-testid="bake-progress" role="status" aria-live="polite">
            {progressLabel(progress)}
          </p>
        )}
        {error && (
          <p className="notice error" data-testid="bake-error" role="alert">
            {error}
          </p>
        )}
        <div className="bake-dialog-actions">
          <button type="button" data-testid="bake-dialog-cancel" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
          <button
            type="button"
            className="primary"
            data-testid="bake-dialog-confirm"
            onClick={confirm}
            disabled={busy || layers == null}
          >
            {busy ? "Baking…" : "Bake"}
          </button>
        </div>
      </div>
    </div>
  );
}
