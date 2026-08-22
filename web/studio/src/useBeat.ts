/**
 * T85 — drives the beat frame from the song's tempo.
 *
 * Timing is read from a monotonic clock (`performance.now`) each frame and each beat's
 * onset is `start + k × interval` — never a chained `setTimeout(interval)`, which
 * accumulates drift under load (the failure mode `beat-phase.vectors` guards on the
 * contract side). The frame style is written straight to a ref inside the rAF loop, so a
 * 60 fps pulse does NOT re-render the editor — only the running/continuous booleans are
 * React state, for the control's pressed look.
 */
import { type RefObject, useCallback, useEffect, useRef, useState } from "react";
import { beatFrameStyle } from "./beatFrame";
import { frameBox } from "./layout";
import { countInUnits, decayMs, meterGroups, unitIntervalMs } from "./beatPhase";

/** A gap outside the page where the frame sits, so the rail never overlaps the sheet. */
const FRAME_GAP_PX = 6;

/**
 * Position the frame at the intersection of the PAGE box and the visible viewport, per side:
 * where a page edge is on-screen the rail hugs it (a wide monitor keeps the beat near the music,
 * not out at the far edges); where the page runs off-screen (zoomed in) that side falls back to
 * the viewport edge. Runs each frame while beating, so it tracks scroll and zoom live.
 */
function positionFrame(frame: HTMLDivElement): void {
  const body = frame.parentElement;
  if (!body) return;
  const bodyRect = body.getBoundingClientRect();
  const scroll = body.querySelector<HTMLElement>(".viewer-scroll");
  const pages = scroll?.querySelectorAll<HTMLElement>(".pdf-page");

  const v = (scroll ?? body).getBoundingClientRect();
  let box: { left: number; top: number; right: number; bottom: number };
  if (pages && pages.length > 0) {
    // The pages share one centred column, so a single page gives left/right; the column's
    // vertical extent runs from the first page's top to the last page's bottom. Two reads
    // regardless of page count — a 20-page part must not force a layout read per page each
    // frame (Fable's T85b nit, and the shape A35 should port to the tablet).
    const first = pages[0].getBoundingClientRect();
    const last = pages[pages.length - 1].getBoundingClientRect();
    box = frameBox(
      { left: first.left, top: first.top, right: first.right, bottom: last.bottom },
      v,
      FRAME_GAP_PX,
    );
  } else {
    // No page yet — frame the viewport with a small inset (the original behaviour).
    box = { left: v.left + 8, top: v.top + 8, right: v.right - 8, bottom: v.bottom - 8 };
  }
  frame.style.left = `${box.left - bodyRect.left}px`;
  frame.style.top = `${box.top - bodyRect.top}px`;
  frame.style.width = `${Math.max(0, box.right - box.left)}px`;
  frame.style.height = `${Math.max(0, box.bottom - box.top)}px`;
}

export interface UseBeat {
  running: boolean;
  continuous: boolean;
  /** Attach to the beat-frame overlay; the rAF loop writes its style imperatively. */
  frameRef: RefObject<HTMLDivElement>;
  /** Start a count-in (default) or a continuous beat. No-op without a tempo. */
  start: (continuous?: boolean) => void;
  stop: () => void;
  setContinuous: (v: boolean) => void;
}

export function useBeat(bpm: number | null | undefined, meter?: string | null): UseBeat {
  const [running, setRunning] = useState(false);
  const [continuous, setContinuousState] = useState(false);
  const frameRef = useRef<HTMLDivElement>(null);
  const rafRef = useRef<number | null>(null);
  const startRef = useRef(0);
  const continuousRef = useRef(false);
  const bpmRef = useRef(bpm ?? 0);
  bpmRef.current = bpm ?? 0;
  const meterRef = useRef(meter ?? "");
  meterRef.current = meter ?? "";

  const clearFrame = useCallback(() => {
    const el = frameRef.current;
    if (!el) return;
    el.style.opacity = "0";
    el.style.boxShadow = "none";
    el.removeAttribute("data-beat");
  }, []);

  const stop = useCallback(() => {
    if (rafRef.current != null) cancelAnimationFrame(rafRef.current);
    rafRef.current = null;
    setRunning(false);
    clearFrame();
  }, [clearFrame]);

  const tick = useCallback(() => {
    const bpmNow = bpmRef.current;
    if (bpmNow <= 0) {
      stop();
      return;
    }
    // T86: the clock ticks on the UNIT, and the count-in is two BARS (in units), both derived
    // from the song's metre. An unset/invalid metre falls back to 4/4.
    const groups = meterGroups(meterRef.current);
    const interval = unitIntervalMs(bpmNow, groups);
    const beats = continuousRef.current ? Infinity : countInUnits(groups);
    const elapsed = performance.now() - startRef.current;
    // A count-in self-stops once its last unit has fully faded.
    if (!continuousRef.current && elapsed >= beats * interval + decayMs(interval)) {
      stop();
      return;
    }
    const el = frameRef.current;
    if (el) {
      const s = beatFrameStyle(elapsed, interval, beats, groups);
      if (s) {
        positionFrame(el); // hug the page where visible, viewport where not
        el.style.borderWidth = `${s.borderWidth}px`;
        el.style.borderColor = s.borderColor;
        el.style.opacity = String(s.opacity);
        el.style.boxShadow = s.boxShadow;
        el.setAttribute("data-beat", String(Math.floor(elapsed / interval)));
      } else {
        el.style.opacity = "0";
        el.style.boxShadow = "none";
      }
    }
    rafRef.current = requestAnimationFrame(tick);
  }, [stop]);

  const start = useCallback(
    (cont = false) => {
      if (bpmRef.current <= 0) return;
      continuousRef.current = cont;
      setContinuousState(cont);
      startRef.current = performance.now();
      setRunning(true);
      if (rafRef.current != null) cancelAnimationFrame(rafRef.current);
      rafRef.current = requestAnimationFrame(tick);
    },
    [tick],
  );

  const setContinuous = useCallback((v: boolean) => {
    continuousRef.current = v;
    setContinuousState(v);
  }, []);

  // Stop the loop if the component unmounts mid-beat.
  useEffect(() => stop, [stop]);

  return { running, continuous, frameRef, start, stop, setContinuous };
}
