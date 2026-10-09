import React from "react";
import { AbsoluteFill, Audio, Freeze, OffthreadVideo, Sequence, interpolate, staticFile, useCurrentFrame, useVideoConfig, Easing } from "remotion";
import { C, ARC } from "./theme";
import { TitleCard, EndCard, CreditsCard, LowerThird, Callout } from "./components";

type Seg = { text: string; start: number; end: number };
type Scene = {
  id: string; start: number; duration: number; card?: "title" | "end" | "credits";
  narration?: { src: string; at: number; duration: number; segments: Seg[] };
  take?: { src: string; from: number; to: number; rate: number; device: "tablet" | "browser" };
  marks?: Record<string, number>;
  lower?: { text: string; at?: number; seg?: number };
  zoom?: { mark: string; rect: number[] };
  callouts?: { seg: number; rect: number[]; label: string }[];
};
export type Plan = {
  id: string; title: string; series: string; arc: string; next: string; fps: number; durationInFrames: number;
  scenes: Scene[]; music: { sting: string; bed: string; outro: string; bedFrom: number; bedTo: number; outroAt: number };
};

const TAB = { w: 1200, h: 1920 };           // the filming tablet's screen, in take pixels
const TAB_H = 960;                           // its on-screen height in the 1080p frame
const TAB_S = TAB_H / TAB.h;
const TAB_W = TAB.w * TAB_S;
const TAB_LEFT = (1920 - TAB_W) / 2, TAB_TOP = (1080 - TAB_H) / 2;

const segAt = (sc: Scene, i: number) => (sc.narration ? sc.narration.at + (sc.narration.segments[i]?.start ?? 0) : 0);
const segLen = (sc: Scene, i: number) => {
  const n = sc.narration; if (!n) return 4;
  const next = n.segments[i + 1]?.start ?? n.duration;
  return Math.max(2.5, next - (n.segments[i]?.start ?? 0) + 0.4);
};

const Clip: React.FC<{ sc: Scene; fps: number }> = ({ sc, fps }) => {
  const t = sc.take!;
  const clipFrames = Math.max(1, Math.floor(((t.to - t.from) / t.rate) * fps));
  const video = <OffthreadVideo src={staticFile(t.src)} startFrom={Math.round(t.from * fps)} playbackRate={t.rate} muted
    style={{ width: "100%", height: "100%" }} />;
  return (
    <>
      <Sequence durationInFrames={clipFrames} layout="none">{video}</Sequence>
      <Sequence from={clipFrames} layout="none"><Freeze frame={clipFrames - 1}>{video}</Freeze></Sequence>
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
  const tx = (960 - cx) * k, ty = (540 - cy) * k;
  return <AbsoluteFill style={{ transform: `translate(${tx}px, ${ty}px) scale(${s})`, transformOrigin: `${cx}px ${cy}px` }}>{children}</AbsoluteFill>;
};

const TakeScene: React.FC<{ sc: Scene; plan: Plan }> = ({ sc, plan }) => {
  const { fps } = useVideoConfig();
  const arc = plan.arc;
  const overlays = (
    <>
      {sc.lower && (
        <Sequence from={Math.round((sc.lower.seg !== undefined ? segAt(sc, sc.lower.seg) : sc.lower.at ?? 1) * fps)} durationInFrames={Math.round(4.5 * fps)} layout="none">
          <LowerThird text={sc.lower.text} arc={arc} />
        </Sequence>
      )}
    </>
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
        {(sc.callouts ?? []).map((c, i) => (
          <Sequence key={i} from={Math.round(segAt(sc, c.seg) * fps)} durationInFrames={Math.round(segLen(sc, c.seg) * fps)} layout="none">
            <Callout rect={c.rect} label={c.label} arc={arc} map={map} len={Math.round(segLen(sc, c.seg) * fps)} />
          </Sequence>
        ))}
        {overlays}
      </AbsoluteFill>
    );
  }
  return (
    <AbsoluteFill style={{ background: "#000" }}>
      <Zoomed sc={sc} fps={fps}><Clip sc={sc} fps={fps} /></Zoomed>
      {overlays}
    </AbsoluteFill>
  );
};

export const Episode: React.FC<{ planUrl: string; plan?: Plan }> = ({ plan }) => {
  const { fps } = useVideoConfig();
  if (!plan) return null;
  const F = (s: number) => Math.round(s * fps);
  const speech = plan.scenes.filter((s) => s.narration).map((s) => [s.start + s.narration!.at, s.start + s.narration!.at + s.narration!.duration]);
  const m = plan.music;
  const bedVolume = (f: number) => {
    const t = f / fps + m.bedFrom;
    const inSpeech = speech.some(([a, b]) => t > a - 0.4 && t < b + 0.4);
    const near = Math.min(...speech.map(([a, b]) => (t < a ? a - t : t > b ? t - b : 0)));
    const duck = inSpeech ? 0.16 : interpolate(near, [0.4, 1.0], [0.16, 0.42], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
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
          {sc.narration && <Sequence from={F(sc.narration.at)} layout="none"><Audio src={staticFile(sc.narration.src)} /></Sequence>}
        </Sequence>
      ))}
      <Audio src={staticFile(m.sting)} volume={0.9} />
      <Sequence from={F(m.bedFrom)} durationInFrames={F(m.bedTo - m.bedFrom)} layout="none">
        <Audio src={staticFile(m.bed)} loop volume={bedVolume} />
      </Sequence>
      <Sequence from={F(m.outroAt)} layout="none"><Audio src={staticFile(m.outro)} volume={0.85} /></Sequence>
    </AbsoluteFill>
  );
};
