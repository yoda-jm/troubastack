/**
 * T203 stage 4b — the Studio sound-sheet view. Go is authoritative: it renders the server's SheetModel (the
 * same model the PDF and CSV render), with an inline per-concert patch editor (admin), a [Stage | Patch] order
 * toggle, "Copy patch from…", "Auto-number", and PDF/CSV download.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError, api, type SheetModel, type SheetOrder, type ChannelPatch, type Setlist } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { useDialogs } from "../components/Dialog";

type Draft = Record<string, { input: string; strip: string }>;

export function SoundSheet() {
  const { bandId, setlistId } = useParams<{ bandId: string; setlistId: string }>();
  const { confirm } = useDialogs();
  const [model, setModel] = useState<SheetModel | null>(null);
  const [order, setOrder] = useState<SheetOrder>("stage");
  const [isAdmin, setIsAdmin] = useState(false);
  const [setlists, setSetlists] = useState<Setlist[]>([]);
  const [draft, setDraft] = useState<Draft>({});
  const [error, setError] = useState<string | null>(null);
  const dirty = useRef<Set<string>>(new Set()); // channel keys with unsaved edits — survive an order toggle

  const load = useCallback(async () => {
    if (!bandId || !setlistId) return;
    try {
      const [m, band, sls] = await Promise.all([
        api.getSheet(bandId, setlistId, order),
        api.getBand(bandId),
        api.listSetlists(bandId),
      ]);
      setModel(m);
      setIsAdmin(band.myRole === "admin");
      setSetlists(sls.filter((s) => s.id !== setlistId));
      // Rebuild the draft from the server, but KEEP any key the user has edited but not saved (so an order
      // toggle — which re-fetches — never discards unsaved patch edits).
      setDraft((prev) => {
        const d: Draft = {};
        for (const sc of m.subcols) {
          if (!sc.amplified) continue;
          d[sc.key] = dirty.current.has(sc.key) && prev[sc.key] ? prev[sc.key] : { input: sc.patchInput ? String(sc.patchInput) : "", strip: sc.patchStrip ?? "" };
        }
        return d;
      });
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't load the sound sheet.");
    }
  }, [bandId, setlistId, order]);
  useEffect(() => {
    void load();
  }, [load]);

  const anyPatched = useMemo(() => (model?.subcols ?? []).some((sc) => sc.amplified && sc.patchInput && sc.patchInput > 0), [model]);
  // Input numbers that appear on more than one channel in the current draft — highlighted, and blocked on save
  // (a console can't patch two sources to one input; the server also refuses, naming both).
  const dupInputs = useMemo(() => {
    const seen = new Map<number, number>();
    for (const sc of model?.subcols ?? []) {
      const n = parseInt(draft[sc.key]?.input ?? "", 10);
      if (!Number.isNaN(n) && n > 0) seen.set(n, (seen.get(n) ?? 0) + 1);
    }
    return new Set([...seen.entries()].filter(([, c]) => c > 1).map(([n]) => n));
  }, [model, draft]);

  async function savePatch() {
    if (!bandId || !setlistId || !model) return;
    const patch: ChannelPatch[] = [];
    for (const sc of model.subcols) {
      if (!sc.amplified) continue;
      const d = draft[sc.key];
      const input = d ? parseInt(d.input, 10) : NaN;
      const strip = (d?.strip ?? "").trim();
      if (!Number.isNaN(input) && input > 0) {
        patch.push({ channel: sc.key, input, name: strip });
      } else if (strip !== "") {
        // A strip name with no input would be silently dropped — block with a clear message (Fable).
        setError(`“${strip}” on ${sc.ownerName} · ${sc.channel} has no input number — add one or clear the name.`);
        return;
      }
    }
    if (dupInputs.size > 0) {
      setError(`Two channels share input ${[...dupInputs].join(", ")} — give each its own number.`);
      return;
    }
    try {
      await api.setPatch(bandId, setlistId, patch);
      dirty.current.clear();
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't save the patch.");
    }
  }
  // Discarding unsaved draft edits needs a confirm.
  async function confirmDiscardIfDirty(): Promise<boolean> {
    if (dirty.current.size === 0) return true;
    return confirm({ title: "Discard unsaved patch edits?", body: "You have patch changes that aren't saved. This replaces them.", confirmLabel: "Discard & continue" });
  }

  async function autoNumber() {
    if (!bandId || !setlistId) return;
    if (!(await confirmDiscardIfDirty())) return;
    // Auto-number renumbers EVERY channel; confirm in-page when any input is already set (Fable 4b note).
    if (anyPatched) {
      const ok = await confirm({ title: "Re-number every channel?", body: "Auto-number assigns 1..N in the current order, replacing the existing input numbers. Strip names you've set are kept.", confirmLabel: "Re-number" });
      if (!ok) return;
    }
    try {
      await api.autoNumberPatch(bandId, setlistId, order);
      dirty.current.clear();
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't auto-number.");
    }
  }
  async function copyFrom(fromId: string) {
    if (!bandId || !setlistId || !fromId) return;
    if (!(await confirmDiscardIfDirty())) return;
    const ok = await confirm({ title: "Copy this patch?", body: "The current patch is replaced with the chosen concert's, matched by channel.", confirmLabel: "Copy" });
    if (!ok) return;
    try {
      await api.copyPatch(bandId, setlistId, fromId);
      dirty.current.clear();
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't copy the patch.");
    }
  }

  if (!bandId || !setlistId) return null;

  return (
    <>
      <p className="crumb">
        <Link to={`/bands/${bandId}/setlists/${setlistId}`}>← Concert</Link>
      </p>
      <h1 data-testid="sheet-title">Sound sheet{model ? ` — ${model.setlistName}` : ""}</h1>
      <ErrorBanner message={error} />

      <div className="sheet-controls">
        <button type="button" className="btn-sm" data-testid="sheet-order" aria-pressed={order === "patch"} onClick={() => setOrder((o) => (o === "stage" ? "patch" : "stage"))}>
          {order === "stage" ? "Order: stage" : "Order: patch"}
        </button>
        <a className="btn-sm" data-testid="sheet-pdf" href={api.sheetPdfUrl(bandId, setlistId, order)} target="_blank" rel="noreferrer">PDF</a>
        <a className="btn-sm" data-testid="sheet-csv" href={api.sheetCsvUrl(bandId, setlistId, order)} target="_blank" rel="noreferrer">CSV</a>
        {isAdmin && (
          <>
            <button type="button" className="btn-sm" data-testid="sheet-autonumber" onClick={() => void autoNumber()}>Auto-number</button>
            <button type="button" className="btn-sm" data-testid="sheet-save-patch" onClick={() => void savePatch()}>Save patch</button>
            {setlists.length > 0 && (
              <select className="btn-sm" data-testid="sheet-copy-from" defaultValue="" onChange={(e) => { const v = e.target.value; e.target.value = ""; void copyFrom(v); }}>
                <option value="">Copy patch from…</option>
                {setlists.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
              </select>
            )}
          </>
        )}
      </div>

      {model && (
        <table className="lineup-grid sheet-grid" data-testid="sheet-grid">
          <thead>
            <tr>
              <th rowSpan={3}>Song</th>
              {model.ownerSpans.map((s, i) => <th key={i} colSpan={s.span} data-testid="sheet-owner">{s.ownerName}</th>)}
              <th rowSpan={3}>Changes</th>
              <th rowSpan={3}>Sound note</th>
            </tr>
            <tr>
              {model.subcols.map((sc) => <th key={sc.key} data-testid="sheet-channel">{sc.channel}</th>)}
            </tr>
            <tr>
              {model.subcols.map((sc) =>
                isAdmin && sc.amplified ? (
                  <th key={sc.key} className="sheet-patch-cell">
                    {(() => {
                      const n = parseInt(draft[sc.key]?.input ?? "", 10);
                      const isDup = !Number.isNaN(n) && n > 0 && dupInputs.has(n);
                      return (
                        <input
                          className={"sheet-in" + (isDup ? " dup" : "")}
                          type="number"
                          min={0}
                          value={draft[sc.key]?.input ?? ""}
                          data-testid="sheet-in"
                          data-dup={isDup ? "1" : undefined}
                          aria-invalid={isDup || undefined}
                          aria-label={`input number for ${sc.ownerName} ${sc.channel}`}
                          onChange={(e) => { dirty.current.add(sc.key); setDraft((d) => ({ ...d, [sc.key]: { input: e.target.value, strip: d[sc.key]?.strip ?? "" } })); }}
                        />
                      );
                    })()}
                    <input className="sheet-strip" placeholder="strip" value={draft[sc.key]?.strip ?? ""} data-testid="sheet-strip" aria-label={`strip name for ${sc.ownerName} ${sc.channel}`}
                      onChange={(e) => { dirty.current.add(sc.key); setDraft((d) => ({ ...d, [sc.key]: { input: d[sc.key]?.input ?? "", strip: e.target.value } })); }} />
                  </th>
                ) : (
                  <th key={sc.key} className="muted" data-testid="sheet-input">{sc.input}</th>
                ),
              )}
            </tr>
          </thead>
          <tbody>
            {model.rows.map((r) => (
              <tr key={r.songId} data-testid="sheet-row">
                <td>{r.position} · {r.title}</td>
                {model.subcols.map((sc) => {
                  const c = r.cells[sc.key];
                  const txt = c?.optional && c.text ? `(${c.text})` : c?.text ?? "";
                  return <td key={sc.key} className="sheet-body-cell">{txt}</td>;
                })}
                <td className="muted" data-testid="sheet-changes">{(r.changes ?? []).join("; ")}</td>
                <td className="muted">{r.soundNote}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {model && model.inputList.length > 0 && (
        <section className="sheet-inputlist" data-testid="sheet-inputlist">
          <h2>Input list</h2>
          <ol>
            {model.inputList.map((l) => (
              <li key={l.key}>In {l.input} · {l.strip || "—"} · {l.who} · {l.part} · {l.mic}</li>
            ))}
          </ol>
          {model.notPatched && model.notPatched.length > 0 && <p className="muted">Not patched yet: {model.notPatched.join("; ")}</p>}
        </section>
      )}
    </>
  );
}
