import React from "react";
import { AbsoluteFill, Audio, Freeze, OffthreadVideo, Sequence, interpolate, staticFile, useCurrentFrame, useVideoConfig, Easing } from "remotion";
import { C, SANS } from "./theme";
import { TitleCard, EndCard, CreditsCard, LowerThird, Callout } from "./components";
import { Terminal, BrowserFrame, StackDiagram, RevealList } from "./scenes2";

type Step = { kind: "play"; from: number; to: number; rate: number } | { kind: "hold"; at: number; seconds: number };
type Voice = { src: string; from: number; to: number; at: number; text: string };
type Scene = {
  id: string; start: number; duration: number; card?: "title" | "end" | "credits" | "diagram-stack" | "list";
  term?: { events: { kind: "cmd" | "out"; text: string; at: number; typing?: number }[]; cols: number; frame: "full" | "small" };
  frame?: { url: string }; urls?: { mark: string; path: string }[];
  heading?: string; items?: { text: string; seg: number }[];
  voice: Voice[]; segs: [number, number][];
  take?: { src: string; timeline: Step[]; device: "tablet" | "browser" };
  marks?: Record<string, number>;
  lower?: { text: string; at?: number; seg?: number };
  zoom?: { mark: string; rect: number[] };
  callouts?: { seg?: number; mark?: string; before?: number; seconds?: number; rect: number[]; label: string }[];
};
export type Plan = {
  id: string; title: string; series: string; arc: string; next: string; fps: number; durationInFrames: number;
  scenes: Scene[]; cues: { from: number; to: number; text: string }[];
  music: { sting: string; bed: string; outro: string; bedFrom: number; bedTo: number; outroAt: number };
};

const TAB = { w: 1200, h: 1920 };           // the filming tablet's screen, in take pixels
const TAB_H = 960;                           // its on-screen height in the 1080p frame
const TAB_S = TAB_H / TAB.h;
const TAB_W = TAB.w * TAB_S;
const TAB_LEFT = (1920 - TAB_W) / 2, TAB_TOP = (1080 - TAB_H) / 2 - 20;

const segStart = (sc: Scene, i: number) => sc.segs[i]?.[0] ?? 0;
const segLen = (sc: Scene, i: number) => {
  const s = sc.segs[i]; if (!s) return 4;
  const next = sc.segs[i + 1]?.[0] ?? s[1] + 0.6;
  return Math.max(2.5, next - s[0] + 0.3);
};

/** The take's timeline: play steps (a source range at a rate) and hold steps (one frame, N seconds). */
const Clip: React.FC<{ sc: Scene; fps: number }> = ({ sc, fps }) => {
  const t = sc.take!;
  let at = 0;
  return (
    <>
      {t.timeline.map((s, i) => {
        const frames = Math.max(1, Math.round((s.kind === "play" ? (s.to - s.from) / s.rate : s.seconds) * fps));
        const from = at;
        at += frames;
        const video = (
          <OffthreadVideo src={staticFile(t.src)} startFrom={Math.round((s.kind === "play" ? s.from : s.at) * fps)}
            playbackRate={s.kind === "play" ? s.rate : 1} muted style={{ width: "100%", height: "100%" }} />
        );
        return (
          <Sequence key={i} from={from} durationInFrames={frames} layout="none">
            {s.kind === "play" ? video : <Freeze frame={0}>{video}</Freeze>}
          </Sequence>
        );
      })}
    </>
  );
};

const Zoomed: React.FC<{ sc: Scene; fps: number; children: React.ReactNode }> = ({ sc, fps, children }) => {
  const f = useCurrentFrame();
  if (!sc.zoom || sc.marks?.[sc.zoom.mark] === undefined) return <>{children}</>;
  const [x0, y0, x1, y1] = sc.zoom.rect;
  const at = (sc.marks[sc.zoom.mark] + 0.6) * fps;
  const k = interpolate(f, [at, at + 1.2 * fps], [0, 1], { extrapolateLeft: "clamp", extrapolateRight: "clamp", easing: Easing.inOut(Easing.cubic) });
  const s = 1 + (Math.min(1.9, 1 / (x1 - x0), 1 / (y1 - y0)) - 1) * k;
  const cx = ((x0 + x1) / 2) * 1920, cy = ((y0 + y1) / 2) * 1080;
  // scale about the top-left corner, aim the region's centre at the frame's centre, then clamp so the
  // scaled picture always covers the frame (no black band at an edge)
  const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));
  const tx = clamp(960 - cx * s, 1920 - 1920 * s, 0);
  const ty = clamp(540 - cy * s, 1080 - 1080 * s, 0);
  return <AbsoluteFill style={{ transform: `translate(${tx}px, ${ty}px) scale(${s})`, transformOrigin: "0 0" }}>{children}</AbsoluteFill>;
};

const TakeScene: React.FC<{ sc: Scene; plan: Plan }> = ({ sc, plan }) => {
  const { fps } = useVideoConfig();
  const arc = plan.arc;
  const lower = sc.lower && (
    <Sequence from={Math.round((sc.lower.seg !== undefined ? segStart(sc, sc.lower.seg) : sc.lower.at ?? 1) * fps)} durationInFrames={Math.round(4.5 * fps)} layout="none">
      <LowerThird text={sc.lower.text} arc={arc} />
    </Sequence>
  );
  if (sc.take!.device === "tablet") {
    const map = (x: number, y: number): [number, number] => [TAB_LEFT + x * TAB_S, TAB_TOP + y * TAB_S];
    return (
      <AbsoluteFill style={{ background: `radial-gradient(70% 80% at 50% 45%, ${C.paper}, ${C.paperAlt})` }}>
        <div style={{ position: "absolute", left: TAB_LEFT - 18, top: TAB_TOP - 18, width: TAB_W + 36, height: TAB_H + 36, borderRadius: 40,
          background: "#0F151B", boxShadow: "0 50px 90px -30px rgba(20,26,31,.55), 0 0 0 1px rgba(255,255,255,.06) inset" }} />
        <div style={{ position: "absolute", left: TAB_LEFT, top: TAB_TOP, width: TAB_W, height: TAB_H, borderRadius: 22, overflow: "hidden", background: "#000" }}>
          <Clip sc={sc} fps={fps} />
        </div>
        {(sc.callouts ?? []).map((c, i) => {
          const start = c.mark !== undefined ? Math.max(0, (sc.marks?.[c.mark] ?? 0) - (c.before ?? 1)) : segStart(sc, c.seg ?? 0);
          const len = c.seconds ?? segLen(sc, c.seg ?? 0);
          return (
            <Sequence key={i} from={Math.round(start * fps)} durationInFrames={Math.round(len * fps)} layout="none">
              <Callout rect={c.rect} label={c.label} arc={arc} map={map} len={Math.round(len * fps)} />
            </Sequence>
          );
        })}
        {lower}
      </AbsoluteFill>
    );
  }
  if (sc.frame) {
    return <FramedTake sc={sc} plan={plan} lower={lower} />;
  }
  return (
    <AbsoluteFill style={{ background: "#000" }}>
      <Zoomed sc={sc} fps={fps}><Clip sc={sc} fps={fps} /></Zoomed>
      {lower}
    </AbsoluteFill>
  );
};

/** A browser take inside a window frame whose address follows the page (the `urls` marks). */
const FramedTake: React.FC<{ sc: Scene; plan: Plan; lower: React.ReactNode }> = ({ sc, lower }) => {
  const f = useCurrentFrame();
  const { fps } = useVideoConfig();
  const t = f / fps;
  let path = "";
  for (const u of sc.urls ?? []) if ((sc.marks?.[u.mark] ?? Infinity) <= t) path = u.path;
  return (
    <>
      <BrowserFrame url={sc.frame!.url + path}><Zoomed sc={sc} fps={fps}><Clip sc={sc} fps={fps} /></Zoomed></BrowserFrame>
      {lower}
    </>
  );
};

/** Burned-in captions: white on a dark translucent bar at the bottom — readable over a white app screen
 *  in any player (VLL: the player's white-on-white subtitles were hard to read and too big). */
const Captions: React.FC<{ plan: Plan }> = ({ plan }) => {
  const f = useCurrentFrame();
  const t = f / plan.fps;
  const cue = plan.cues.find((c) => t >= c.from && t < c.to);
  if (!cue) return null;
  return (
    <div style={{ position: "absolute", left: 0, right: 0, bottom: 34, display: "flex", justifyContent: "center", pointerEvents: "none" }}>
      <div style={{ maxWidth: 1240, background: "rgba(14,18,22,.78)", color: "#F4F1EA", fontFamily: SANS, fontSize: 32, lineHeight: 1.32,
        fontWeight: 500, padding: "8px 22px 10px", borderRadius: 10, textAlign: "center", whiteSpace: "pre-line",
        textShadow: "0 1px 2px rgba(0,0,0,.6)" }}>
        {cue.text}
      </div>
    </div>
  );
};

export const Episode: React.FC<{ planUrl: string; plan?: Plan }> = ({ plan }) => {
  const { fps } = useVideoConfig();
  if (!plan) return null;
  const F = (s: number) => Math.round(s * fps);
  const speech = plan.scenes.flatMap((s) => s.segs.map(([a, b]) => [s.start + a, s.start + b]));
  const m = plan.music;
  const bedVolume = (f: number) => {
    const t = f / fps + m.bedFrom;
    const near = Math.min(...speech.map(([a, b]) => (t < a ? a - t : t > b ? t - b : 0)));
    const duck = interpolate(near, [0.4, 1.0], [0.16, 0.42], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
    const edge = Math.min(interpolate(t, [m.bedFrom, m.bedFrom + 2], [0, 1], { extrapolateRight: "clamp" }),
      interpolate(t, [m.bedTo - 2, m.bedTo], [1, 0], { extrapolateLeft: "clamp" }));
    return duck * edge;
  };
  return (
    <AbsoluteFill style={{ background: C.tile }}>
      {plan.scenes.map((sc) => (
        <Sequence key={sc.id} from={F(sc.start)} durationInFrames={F(sc.duration)}>
          {sc.card === "title" && <TitleCard num={plan.id} title={plan.title} series={plan.series} arc={plan.arc} len={F(sc.duration)} />}
          {sc.card === "end" && <EndCard next={plan.next} arc={plan.arc} len={F(sc.duration)} />}
          {sc.card === "credits" && <CreditsCard len={F(sc.duration)} />}
          {sc.take && <TakeScene sc={sc} plan={plan} />}
          {sc.term && <Terminal events={sc.term.events} cols={sc.term.cols} small={sc.term.frame === "small"} />}
          {sc.card === "diagram-stack" && <StackDiagram segs={sc.segs} />}
          {sc.card === "list" && <RevealList heading={sc.heading ?? ""} items={sc.items ?? []} segs={sc.segs} arc={plan.arc} />}
          {(sc.term || sc.card === "diagram-stack" || sc.card === "list") && sc.lower && (
            <Sequence from={F(sc.lower.seg !== undefined ? (sc.segs[sc.lower.seg]?.[0] ?? 0) : sc.lower.at ?? 1)} durationInFrames={F(4.5)} layout="none">
              <LowerThird text={sc.lower.text} arc={plan.arc} />
            </Sequence>
          )}
          {sc.voice.map((v, i) => (
            <Sequence key={i} from={F(v.at)} durationInFrames={Math.max(1, F(v.to - v.from) + 2)} layout="none">
              <Audio src={staticFile(v.src)} startFrom={F(v.from)} endAt={F(v.to) + 2} />
            </Sequence>
          ))}
        </Sequence>
      ))}
      <Captions plan={plan} />
      <Audio src={staticFile(m.sting)} volume={0.9} />
      <Sequence from={F(m.bedFrom)} durationInFrames={F(m.bedTo - m.bedFrom)} layout="none">
        <Audio src={staticFile(m.bed)} loop volume={bedVolume} />
      </Sequence>
      <Sequence from={F(m.outroAt)} layout="none"><Audio src={staticFile(m.outro)} volume={0.85} /></Sequence>
    </AbsoluteFill>
  );
};
