/**
 * My-cues editor panel (T50): the caller's PERSONAL set of icon+color cues for a
 * song — what they must prepare ("mic + red guitar-electric"). Self-only; changes
 * apply immediately (PUT /my-cues). Add from the labelled glyph picker, retint from
 * the fixed stage palette, reorder, remove. Capped at 4 for glanceability.
 */
import { useCallback, useEffect, useState } from "react";
import { api, type SongCue } from "../../api";
import { CueGlyph, CUE_ICON_IDS, CUE_ICON_LABELS } from "../../components/CueGlyphs";

// The fixed stage palette: a neutral (inherit) plus 8 tints that stay legible on
// both day and night surfaces. The model accepts any hex; the UI offers these.
const CUE_PALETTE: { name: string; value: string }[] = [
  { name: "Neutral", value: "" },
  { name: "Red", value: "#e11d48" },
  { name: "Orange", value: "#ea580c" },
  { name: "Amber", value: "#d97706" },
  { name: "Green", value: "#16a34a" },
  { name: "Teal", value: "#0d9488" },
  { name: "Blue", value: "#2563eb" },
  { name: "Violet", value: "#7c3aed" },
  { name: "Pink", value: "#db2777" },
];

const MAX_CUES = 4;

export function MyCuesEditor({
  bandId,
  songId,
  onError,
}: {
  bandId: string;
  songId: string;
  onError?: (msg: string | null) => void;
}) {
  const [cues, setCues] = useState<SongCue[]>([]);
  const [busy, setBusy] = useState(false);
  // The tint applied to the NEXT glyph added from the picker.
  const [addColor, setAddColor] = useState<string>("");
  const report = useCallback((m: string | null) => onError?.(m), [onError]);

  // Load the caller's saved cues once.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const mine = await api.getMyCues(bandId, songId);
        if (!cancelled) setCues(mine);
      } catch (err) {
        if (!cancelled) report(err instanceof Error ? err.message : "Failed to load cues");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bandId, songId, report]);

  // Persist a new list and adopt the server's cleaned response.
  const apply = useCallback(
    async (next: SongCue[]) => {
      setBusy(true);
      report(null);
      try {
        const saved = await api.setMyCues(bandId, songId, next);
        setCues(saved);
      } catch (err) {
        report(err instanceof Error ? err.message : "Failed to save cues");
      } finally {
        setBusy(false);
      }
    },
    [bandId, songId, report],
  );

  function addCue(icon: string) {
    if (cues.length >= MAX_CUES) return;
    void apply([...cues, { icon, color: addColor || undefined }]);
  }

  function removeCue(index: number) {
    void apply(cues.filter((_, i) => i !== index));
  }

  function retint(index: number, color: string) {
    void apply(cues.map((c, i) => (i === index ? { ...c, color: color || undefined } : c)));
  }

  function move(index: number, dir: -1 | 1) {
    const other = index + dir;
    if (other < 0 || other >= cues.length) return;
    const next = [...cues];
    [next[index], next[other]] = [next[other], next[index]];
    void apply(next);
  }

  const full = cues.length >= MAX_CUES;

  return (
    <section className="my-cues-panel card" data-testid="my-cues-panel">
      <div className="my-cues-panel-head">
        <h2>My cues</h2>
        <span className="muted my-cues-count" data-testid="my-cues-count">
          {cues.length}/{MAX_CUES}
        </span>
      </div>
      <p className="muted my-cues-hint">
        Icons + colors to remind you what to prepare for this song. Just for you — each
        member sets their own.
      </p>

      {/* Current cues. */}
      {cues.length > 0 && (
        <ul className="list my-cues-list">
          {cues.map((c, i) => (
            <li key={`${c.icon}-${i}`} data-testid="cue-chip" data-icon={c.icon} className="cue-chip">
              <span className="cue-chip-glyph">
                <CueGlyph icon={c.icon ?? ""} color={c.color} size={24} />
              </span>
              <span className="cue-chip-label">{CUE_ICON_LABELS[c.icon as keyof typeof CUE_ICON_LABELS] ?? c.icon}</span>
              <span className="cue-chip-tints" role="group" aria-label="Tint">
                {CUE_PALETTE.map((p) => (
                  <button
                    key={p.value || "neutral"}
                    type="button"
                    className={`cue-swatch${(c.color ?? "") === p.value ? " active" : ""}`}
                    data-testid="cue-tint"
                    data-color={p.value}
                    title={p.name}
                    aria-label={p.name}
                    aria-pressed={(c.color ?? "") === p.value}
                    disabled={busy}
                    style={p.value ? { background: p.value } : undefined}
                    onClick={() => retint(i, p.value)}
                  />
                ))}
              </span>
              <span className="cue-chip-actions actions">
                <button type="button" data-testid="cue-up" disabled={busy || i === 0} onClick={() => move(i, -1)}>
                  ↑
                </button>
                <button
                  type="button"
                  data-testid="cue-down"
                  disabled={busy || i === cues.length - 1}
                  onClick={() => move(i, 1)}
                >
                  ↓
                </button>
                <button type="button" data-testid="cue-remove" disabled={busy} onClick={() => removeCue(i)}>
                  ✕
                </button>
              </span>
            </li>
          ))}
        </ul>
      )}

      {/* Tint for the next added cue. */}
      <div className="my-cues-addcolor" role="group" aria-label="Color for new cue">
        {CUE_PALETTE.map((p) => (
          <button
            key={p.value || "neutral"}
            type="button"
            className={`cue-swatch${addColor === p.value ? " active" : ""}`}
            data-testid="cue-addcolor"
            data-color={p.value}
            title={p.name}
            aria-label={p.name}
            aria-pressed={addColor === p.value}
            disabled={busy}
            style={p.value ? { background: p.value } : undefined}
            onClick={() => setAddColor(p.value)}
          />
        ))}
      </div>

      {/* Glyph picker. */}
      <div className="my-cues-picker" role="group" aria-label="Add a cue">
        {CUE_ICON_IDS.map((id) => (
          <button
            key={id}
            type="button"
            className="cue-pick"
            data-testid={`cue-add-${id}`}
            title={CUE_ICON_LABELS[id]}
            aria-label={`Add ${CUE_ICON_LABELS[id]}`}
            disabled={busy || full}
            onClick={() => addCue(id)}
          >
            <CueGlyph icon={id} color={addColor || undefined} size={22} />
            <span className="cue-pick-label">{CUE_ICON_LABELS[id]}</span>
          </button>
        ))}
      </div>
      {full && (
        <p className="muted my-cues-hint" data-testid="my-cues-full">
          Up to {MAX_CUES} cues — remove one to add another.
        </p>
      )}
    </section>
  );
}
