/**
 * P205 Stage 1 — bake dialog. Baking captures which layers are ON by default in the
 * bundle (view-time default). Fable's ruling: NEVER capture silently — show it
 * explicitly ("Baking with: Cues ✓ · Form ✓ · My notes ✗ — edit?"), WYSIWYG seeded
 * from the layers' current default-visibility, editable, remembered per setlist. On
 * confirm the choice rides to the baker as `layerDefaults` (name → default-on).
 */
import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import { AudienceTag, audienceForZone } from "../components/AudienceTag";

type LayerRow = { name: string; zone: string; mandatory: boolean };

function rememberKey(setlistId: string) {
  return `trouba_bake_defaults_${setlistId}`;
}

export function BakeDialog({
  bandId,
  setlistId,
  songIds,
  scope,
  onConfirm,
  onCancel,
}: {
  bandId: string;
  setlistId: string;
  songIds: string[];
  scope?: "mine";
  onConfirm: (layerDefaults: Record<string, boolean>) => void;
  onCancel: () => void;
}) {
  const [layers, setLayers] = useState<LayerRow[] | null>(null);
  const [on, setOn] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false);

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

  const title = scope === "mine" ? "Bake my parts" : "Bake setlist";
  const summary = useMemo(
    () => (layers ?? []).map((l) => `${l.name} ${on[l.name] ? "✓" : "✗"}`).join(" · "),
    [layers, on],
  );

  function toggle(name: string) {
    setOn((s) => ({ ...s, [name]: !s[name] }));
  }
  function confirm() {
    setBusy(true);
    try {
      sessionStorage.setItem(rememberKey(setlistId), JSON.stringify(on));
    } catch {
      /* ignore */
    }
    onConfirm(on);
  }

  return (
    <div className="bake-dialog-backdrop" data-testid="bake-dialog" role="dialog" aria-modal="true" aria-label={title}>
      <div className="bake-dialog card">
        <div className="card-head">
          <h2>{title}</h2>
          <AudienceTag audience={scope === "mine" ? "mine" : "band"} />
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
                      disabled={l.mandatory}
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
