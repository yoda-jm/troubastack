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
import { COUNT_IN_BEATS, decayMs, intervalMs as bpmToIntervalMs } from "./beatPhase";

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

export function useBeat(bpm: number | null | undefined): UseBeat {
  const [running, setRunning] = useState(false);
  const [continuous, setContinuousState] = useState(false);
  const frameRef = useRef<HTMLDivElement>(null);
  const rafRef = useRef<number | null>(null);
  const startRef = useRef(0);
  const continuousRef = useRef(false);
  const bpmRef = useRef(bpm ?? 0);
  bpmRef.current = bpm ?? 0;

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
    const interval = bpmToIntervalMs(bpmNow);
    const beats = continuousRef.current ? Infinity : COUNT_IN_BEATS;
    const elapsed = performance.now() - startRef.current;
    // A count-in self-stops once its last beat has fully faded.
    if (!continuousRef.current && elapsed >= beats * interval + decayMs(interval)) {
      stop();
      return;
    }
    const el = frameRef.current;
    if (el) {
      const s = beatFrameStyle(elapsed, interval, beats);
      if (s) {
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
