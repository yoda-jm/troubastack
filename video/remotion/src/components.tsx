import React from "react";
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig, Easing } from "remotion";
import { C, ARC, SERIF, SANS, MONO } from "./theme";

const fade = (f: number, len: number, inF = 12, outF = 12) =>
  Math.min(interpolate(f, [0, inF], [0, 1], { extrapolateRight: "clamp" }), interpolate(f, [len - outF, len], [1, 0], { extrapolateLeft: "clamp" }));

const Staff: React.FC<{ opacity?: number }> = ({ opacity = 0.14 }) => (
  <svg viewBox="0 0 1920 1080" style={{ position: "absolute", inset: 0, opacity }}>
    <g stroke="#fff" strokeWidth={1.6} transform="rotate(-6 960 540)">
      {[0, 1, 2, 3, 4].map((i) => <line key={i} x1={-200} y1={440 + i * 44} x2={2120} y2={460 + i * 46} />)}
    </g>
  </svg>
);

export const DarkGround: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <AbsoluteFill style={{ background: `radial-gradient(110% 90% at 30% 20%, rgba(58,74,89,.55), rgba(0,0,0,.3)), linear-gradient(150deg, ${C.tileHi}, ${C.tileLo})` }}>
    <Staff />
    {children}
  </AbsoluteFill>
);

/** TitleCard — "03 · Get the app on your tablet", the series name under it. */
export const TitleCard: React.FC<{ num: string; title: string; series: string; arc: string; len: number }> = ({ num, title, series, arc, len }) => {
  const f = useCurrentFrame();
  const { fps } = useVideoConfig();
  const rise = spring({ frame: f, fps, config: { damping: 200 } });
  return (
    <DarkGround>
      <AbsoluteFill style={{ opacity: fade(f, len, 14, 16), justifyContent: "center", paddingLeft: 180 }}>
        <div style={{ transform: `translateY(${(1 - rise) * 24}px)` }}>
          <div style={{ fontFamily: MONO, fontSize: 30, letterSpacing: "0.2em", color: ARC[arc], marginBottom: 26 }}>EPISODE {num}</div>
          <div style={{ fontFamily: SERIF, fontWeight: 700, fontSize: 96, color: C.onDark, lineHeight: 1.05, maxWidth: 1400 }}>{title}</div>
          <div style={{ height: 6, width: interpolate(f, [8, 40], [0, 420], { extrapolateLeft: "clamp", extrapolateRight: "clamp", easing: Easing.out(Easing.cubic) }), background: ARC[arc], borderRadius: 3, margin: "34px 0 30px" }} />
          <div style={{ fontFamily: SANS, fontSize: 34, color: C.onDarkSoft }}>{series}</div>
        </div>
      </AbsoluteFill>
    </DarkGround>
  );
};

/** EndCard — "Next: 04 · …". */
export const EndCard: React.FC<{ next: string; arc: string; len: number }> = ({ next, arc, len }) => {
  const f = useCurrentFrame();
  return (
    <DarkGround>
      <AbsoluteFill style={{ opacity: fade(f, len, 14, 20), justifyContent: "center", paddingLeft: 180 }}>
        <div style={{ fontFamily: MONO, fontSize: 30, letterSpacing: "0.2em", color: ARC[arc], marginBottom: 24 }}>NEXT</div>
        <div style={{ fontFamily: SERIF, fontWeight: 700, fontSize: 80, color: C.onDark, maxWidth: 1450, lineHeight: 1.1 }}>{next}</div>
      </AbsoluteFill>
    </DarkGround>
  );
};

/** CreditsCard — music + the demo charts' licences (NOTICE). */
export const CreditsCard: React.FC<{ len: number }> = ({ len }) => {
  const f = useCurrentFrame();
  const row = (k: string, v: string) => (
    <div style={{ display: "flex", gap: 30, fontSize: 30, marginBottom: 18 }}>
      <div style={{ width: 260, color: C.onDarkSoft, fontFamily: MONO, fontSize: 24, letterSpacing: "0.12em", paddingTop: 6 }}>{k}</div>
      <div style={{ color: C.onDark, maxWidth: 1200 }}>{v}</div>
    </div>
  );
  return (
    <DarkGround>
      <AbsoluteFill style={{ opacity: fade(f, len, 12, 18), justifyContent: "center", paddingLeft: 180, fontFamily: SANS }}>
        <div style={{ fontFamily: SERIF, fontWeight: 700, fontSize: 56, color: C.onDark, marginBottom: 40 }}>TroubaStack</div>
        {row("MUSIC", "Composed for TroubaStack — written as code, rendered with FluidR3_GM (MIT)")}
        {row("VOICE", "Kokoro-82M (Apache-2.0), voice am_eric")}
        {row("DEMO CHARTS", "Original and public-domain songs; Greensleeves after Mutopia, CC BY-SA 4.0 — see NOTICE")}
        {row("SOFTWARE", "Apache-2.0 · github.com/yoda-jm/troubastack")}
      </AbsoluteFill>
    </DarkGround>
  );
};

/** LowerThird — a label sliding in bottom-left, with the arc colour bar. */
export const LowerThird: React.FC<{ text: string; arc: string; len?: number }> = ({ text, arc, len = 4.5 * 30 }) => {
  const f = useCurrentFrame();
  const { fps } = useVideoConfig();
  const inS = spring({ frame: f, fps, config: { damping: 18, mass: 0.6 } });
  const out = interpolate(f, [len - 10, len], [1, 0], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
  return (
    <div style={{ position: "absolute", left: 64, top: 56, opacity: out, transform: `translateX(${(1 - inS) * -60}px)`,
      display: "flex", alignItems: "stretch", background: "rgba(20,26,31,.86)", borderRadius: 14, overflow: "hidden",
      boxShadow: "0 18px 40px -16px rgba(0,0,0,.6)" }}>
      <div style={{ width: 10, background: ARC[arc] }} />
      <div style={{ padding: "14px 26px 16px 20px", fontFamily: SANS, fontWeight: 650, fontSize: 34, color: C.onDark }}>{text}</div>
    </div>
  );
};

/** Callout — a ring around a screen rect (in the TAKE's pixel space, mapped by `map`) with a tag. */
export const Callout: React.FC<{ rect: number[]; label: string; arc: string; map: (x: number, y: number) => [number, number]; len?: number }> = ({ rect, label, arc, map, len = 4 * 30 }) => {
  const f = useCurrentFrame();
  const { fps } = useVideoConfig();
  const [x0, y0] = map(rect[0], rect[1]);
  const [x1, y1] = map(rect[2], rect[3]);
  const s = spring({ frame: f, fps, config: { damping: 14, mass: 0.7 } });
  const out = interpolate(f, [len - 10, len], [1, 0], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
  const pad = 10;
  return (
    <div style={{ position: "absolute", left: x0 - pad, top: y0 - pad, width: x1 - x0 + 2 * pad, height: y1 - y0 + 2 * pad, opacity: out * s,
      transform: `scale(${1.08 - 0.08 * s})`, border: `5px solid ${ARC[arc]}`, borderRadius: 18, boxShadow: `0 0 0 9999px rgba(10,14,18,${0.28 * s})` }}>
      <div style={{ position: "absolute", left: -5, top: -58, background: ARC[arc], color: "#21180A", fontFamily: SANS, fontWeight: 700,
        fontSize: 30, padding: "8px 18px", borderRadius: 10, whiteSpace: "nowrap" }}>{label}</div>
    </div>
  );
};
