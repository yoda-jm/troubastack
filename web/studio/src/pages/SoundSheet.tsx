/**
 * T203 stage 4b — the Studio sound-sheet view. Go is authoritative: it renders the server's SheetModel (the
 * same model the PDF and CSV render), with an inline per-concert patch editor (admin), a [Stage | Patch] order
 * toggle, "Copy patch from…", "Auto-number", and PDF/CSV download.
 */
import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ApiError, api, type SheetModel, type SheetOrder, type ChannelPatch, type Setlist, type MusicianView } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { useDialogs } from "../components/Dialog";
import { LineupScroll } from "../components/LineupScroll";
import { useConcert } from "./ConcertLayout";

type Draft = Record<string, { input: string; strip: string }>;

// T206: the Sound sheet tab. The shared concert header + crumb + tab strip live in ConcertLayout; this tab
// keeps only its own sheet model + patch editing. Role comes from the shared context (no second getBand).
export function SoundSheet() {
  const { bandId, setlistId, myRole } = useConcert();
  const { confirm } = useDialogs();
  const [model, setModel] = useState<SheetModel | null>(null);
  const [order, setOrder] = useState<SheetOrder>("stage");
  const isAdmin = myRole === "admin";
  const [setlists, setSetlists] = useState<Setlist[]>([]);
  const [draft, setDraft] = useState<Draft>({});
  const [error, setError] = useState<string | null>(null);
  // T210: the band roster (names + the band-hidden flag) and this concert's "not at this concert" ids, for the
  // two restore bars and the "not at this concert" control. Fetched here rather than read from the concert
  // context, so they refresh when THIS page changes them.
  const [musicians, setMusicians] = useState<MusicianView[]>([]);
  const [absent, setAbsent] = useState<string[]>([]);
  const dirty = useRef<Set<string>>(new Set()); // channel keys with unsaved edits — survive an order toggle

  const load = useCallback(async () => {
    if (!bandId || !setlistId) return;
    try {
      const [m, sls, mus, sl] = await Promise.all([
        api.getSheet(bandId, setlistId, order),
        api.listSetlists(bandId),
        api.listMusicians(bandId),
        api.getSetlist(bandId, setlistId),
      ]);
      setModel(m);
      setSetlists(sls.filter((s) => s.id !== setlistId));
      setMusicians(mus);
      setAbsent(sl.setlist.absentMusicians ?? []);
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

  // T210: concert scope — "not at this concert" marks/clears a musician for THIS setlist only (admin-only; the
  // server gates it and drops any id that's no longer a musician).
  async function setConcertAbsent(next: string[]) {
    if (!bandId || !setlistId) return;
    try {
      await api.updateSetlist(bandId, setlistId, { absentMusicians: next });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't change who's at this concert.");
    }
  }
  // T210: band scope — clear a musician's band-wide hidden flag from here (the bar's label says it's band-wide,
  // so un-hiding the whole band from a concert page isn't a surprise). Admin-only.
  async function setBandHidden(id: string, hidden: boolean) {
    try {
      await api.updateMusician(bandId, id, { hidden });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't change who's hidden.");
    }
  }

  // The display name for a musician id (for the restore bars); a stale absent id still shows, so an admin can
  // clear it, and falls back to the id rather than a blank.
  const nameOf = (id: string) => musicians.find((m) => m.id === id)?.displayName ?? id;
  const bandHidden = musicians.filter((m) => m.hidden);
  // Visible musicians not yet absent — the "Not at this concert…" select, which is the only way to mark a
  // player who has no personal column of their own (someone who plays only a SHARED part). Hidden ⇒ not offered.
  const absentable = musicians.filter((m) => !m.hidden && !absent.includes(m.id));
  // Each owner span's id + subcol kind, read from the subcols it spans (ownerSpans carries only the name). The
  // header "not here" shortcut is only valid on a CHANNEL span, where ownerId is the MUSICIAN id; a shared span's
  // ownerId is the PART id (sanitizeAbsent would drop it), so no button there — the select handles those.
  const ownerForSpan: { id: string; kind: string }[] = [];
  if (model) {
    let off = 0;
    for (const s of model.ownerSpans) {
      const sc = model.subcols[off];
      ownerForSpan.push({ id: sc?.ownerId ?? "", kind: sc?.kind ?? "" });
      off += s.span;
    }
  }

  return (
    <>
      <ErrorBanner message={error} />

      <div className="sheet-controls">
        <button type="button" className="btn-sm" data-testid="sheet-order" aria-pressed={order === "patch"} onClick={() => setOrder((o) => (o === "stage" ? "patch" : "stage"))}>
          {order === "stage" ? "Order: stage" : "Order: patch"}
        </button>
        <a className="btn-sm" data-testid="sheet-pdf" href={api.sheetPdfUrl(bandId, setlistId, order)} target="_blank" rel="noreferrer">Sound sheet PDF</a>
        <a className="btn-sm" data-testid="sheet-csv" href={api.sheetCsvUrl(bandId, setlistId, order)} target="_blank" rel="noreferrer">Sound sheet CSV</a>
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

      {/* T210: the admin "Not at this concert…" select — the general way to mark someone absent, and the ONLY
          way for a player with no personal column (they play only a shared part, so the header shortcut can't
          reach them). Lists visible musicians not already absent. */}
      {isAdmin && absentable.length > 0 && (
        <div className="sheet-absent-add" data-testid="sheet-absent-add-wrap">
          <label>
            Not at this concert:{" "}
            <select
              className="btn-sm"
              data-testid="sheet-absent-add"
              value=""
              onChange={(e) => { const v = e.target.value; if (v) void setConcertAbsent([...absent, v]); }}
            >
              <option value="">add a player…</option>
              {absentable.map((m) => <option key={m.id} value={m.id}>{m.displayName}</option>)}
            </select>
          </label>
        </div>
      )}

      {/* T210: restore bars. They show for EVERY member whenever anyone is dropped from this sheet, so a missing
          column is never silent; only an admin gets the ↺. Two scopes: "not at this concert" (this setlist) and
          "hidden from the band" (band-wide — the label says so). Outside the scroll box, so they scroll with the
          page. */}
      {absent.length > 0 && (
        <div className="lineup-hidden-bar" data-testid="sheet-absent-bar">
          <span className="lineup-hidden-label">Not at this concert:</span>{" "}
          {absent.map((id, i) => (
            <span key={id} className="lineup-hidden-name" data-testid="sheet-absent-name">
              {i > 0 ? " · " : ""}
              {nameOf(id)}
              {isAdmin && (
                <button type="button" className="linklike lineup-unhide" data-testid="sheet-absent-restore" title={`Bring ${nameOf(id)} back to this concert`} aria-label={`Bring ${nameOf(id)} back to this concert`} onClick={() => void setConcertAbsent(absent.filter((x) => x !== id))}>↺</button>
              )}
            </span>
          ))}
        </div>
      )}
      {bandHidden.length > 0 && (
        <div className="lineup-hidden-bar" data-testid="sheet-bandhidden-bar">
          <span className="lineup-hidden-label">Hidden from the band:</span>{" "}
          {bandHidden.map((m, i) => (
            <span key={m.id} className="lineup-hidden-name" data-testid="sheet-bandhidden-name">
              {i > 0 ? " · " : ""}
              {m.displayName}
              {isAdmin && (
                <button type="button" className="linklike lineup-unhide" data-testid="sheet-bandhidden-restore" title={`Bring ${m.displayName} back to the whole band`} aria-label={`Bring ${m.displayName} back to the whole band`} onClick={() => void setBandHidden(m.id, false)}>↺</button>
              )}
            </span>
          ))}
        </div>
      )}

      {model && (
        <LineupScroll>
          <table className="lineup-grid sheet-grid" data-testid="sheet-grid">
            <thead>
              <tr>
                <th rowSpan={3} className="lineup-corner">Song</th>
                {model.ownerSpans.map((s, i) => (
                  <th key={i} colSpan={s.span} data-testid="sheet-owner">
                    {s.ownerName}
                    {/* T210: drop this owner from THIS concert's sheet only. Only a CHANNEL span is a single
                        musician (ownerId = musician id); a shared span's ownerId is a part id, so no button there
                        — the "Not at this concert…" select above reaches those players. The bar is the way back. */}
                    {isAdmin && ownerForSpan[i]?.kind === "channel" && ownerForSpan[i].id && (
                      <button
                        type="button"
                        className="linklike lineup-hide"
                        data-testid="sheet-owner-absent"
                        title={`${s.ownerName} is not at this concert`}
                        aria-label={`${s.ownerName} is not at this concert`}
                        onClick={() => void setConcertAbsent([...absent, ownerForSpan[i].id])}
                      >
                        not here
                      </button>
                    )}
                  </th>
                ))}
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
              {model.rows.map((r, i) => {
                const fullSpan = 1 + model.subcols.length + 2;
                if (r.kind === "intermission") {
                  return (
                    <tr key={`int-${i}`} data-testid="sheet-intermission">
                      <td colSpan={fullSpan} className="sheet-intermission">— {r.label || "Intermission"} —</td>
                    </tr>
                  );
                }
                const firstOnCall = r.section === "oncall" && (i === 0 || model.rows[i - 1].section !== "oncall");
                return (
                  <Fragment key={r.songId || `row-${i}`}>
                    {firstOnCall && (
                      <tr key={`oncall-${i}`} data-testid="sheet-oncall-head">
                        <td colSpan={fullSpan} className="sheet-oncall-head">On call</td>
                      </tr>
                    )}
                    <tr key={r.songId} data-testid="sheet-row" data-section={r.section}>
                      <td className="lineup-songcell">{r.number ? `${r.number} · ` : ""}{r.title}</td>
                      {model.subcols.map((sc) => {
                        const c = r.cells?.[sc.key];
                        const txt = c?.optional && c.text ? `(${c.text})` : c?.text ?? "";
                        return <td key={sc.key} className="sheet-body-cell">{txt}</td>;
                      })}
                      <td className="muted" data-testid="sheet-changes">{(r.changes ?? []).join("; ")}</td>
                      <td className="muted">{r.soundNote}</td>
                    </tr>
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </LineupScroll>
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
