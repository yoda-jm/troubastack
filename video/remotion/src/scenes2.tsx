import React from "react";
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig, Easing } from "remotion";
import { C, ARC, SERIF, SANS, MONO } from "./theme";
import { DarkGround } from "./components";

/** TERMINAL — replays a cast: each command typed out, then its real output streamed in at its time. */
export const Terminal: React.FC<{ events: { kind: "cmd" | "out"; text: string; at: number; typing?: number }[]; cols: number; small?: boolean }> = ({ events, small }) => {
  const f = useCurrentFrame();
  const { fps } = useVideoConfig();
  const t = f / fps;
  const lines: { text: string; cmd: boolean; cursor?: boolean }[] = [];
  for (const e of events) {
    if (e.at > t) break;
    if (e.kind === "cmd") {
      const shown = Math.min(e.text.length, Math.floor(((t - e.at) / (e.typing ?? 1)) * e.text.length));
      e.text.slice(0, shown).split("\n").forEach((l, i) => lines.push({ text: (i === 0 ? "$ " : "  ") + l, cmd: true }));
      if (shown < e.text.length) lines[lines.length - 1].cursor = true;
    } else {
      e.text.replace(/\n$/, "").split("\n").forEach((l) => lines.push({ text: l, cmd: false }));
    }
  }
  const last = events.filter((e) => e.at <= t).pop();
  if (!lines.length || (last && last.kind === "out")) lines.push({ text: "$ ", cmd: true, cursor: true });
  const rows = small ? 15 : 19;
  const shownLines = lines.slice(-rows);
  const blink = Math.floor(t * 2) % 2 === 0;
  const W = small ? 1240 : 1560, H = small ? 640 : 820;
  return (
    <AbsoluteFill style={{ background: `radial-gradient(80% 90% at 50% 40%, ${C.tileHi}, ${C.tileLo})`, alignItems: "center", justifyContent: "center" }}>
      <div style={{ width: W, height: H, borderRadius: 18, background: "#11171D", boxShadow: "0 50px 90px -30px rgba(0,0,0,.7), 0 0 0 1px rgba(255,255,255,.07) inset", overflow: "hidden", marginTop: -30 }}>
        <div style={{ height: 46, display: "flex", alignItems: "center", gap: 10, padding: "0 18px", background: "#1A222A" }}>
          {["#E5604D", "#E4B03C", "#58B864"].map((c) => <div key={c} style={{ width: 14, height: 14, borderRadius: 7, background: c, opacity: 0.85 }} />)}
          <div style={{ marginLeft: 16, fontFamily: MONO, fontSize: 20, color: C.onDarkSoft }}>~/deploy</div>
        </div>
        <div style={{ padding: "22px 30px", fontFamily: MONO, fontSize: small ? 26 : 29, lineHeight: 1.42, color: "#E8E4DA", whiteSpace: "pre" }}>
          {shownLines.map((l, i) => (
            <div key={i} style={{ color: l.cmd ? C.onDark : "#AEB8C2" }}>
              {l.cmd ? <><span style={{ color: C.core }}>{l.text.slice(0, 2)}</span>{l.text.slice(2)}</> : l.text}
              {l.cursor && blink && <span style={{ background: C.onDark, display: "inline-block", width: "0.6em", height: "1.1em", verticalAlign: "-0.15em" }} />}
            </div>
          ))}
        </div>
      </div>
    </AbsoluteFill>
  );
};

/** A browser window around a browser take: an address bar with the HTTPS padlock and the page's address. */
export const BrowserFrame: React.FC<{ url: string; children: React.ReactNode }> = ({ url, children }) => {
  const W = 1600, H = 900, bar = 54;
  const host = url.replace(/^https:\/\//, "");
  return (
    <AbsoluteFill style={{ background: `radial-gradient(70% 80% at 50% 45%, ${C.paper}, ${C.paperAlt})`, alignItems: "center", justifyContent: "center" }}>
      <div style={{ width: W, height: H + bar, borderRadius: 16, overflow: "hidden", background: "#fff", boxShadow: "0 50px 90px -30px rgba(20,26,31,.45), 0 0 0 1px rgba(20,26,31,.12)", marginTop: -36 }}>
        <div style={{ height: bar, background: "#EEEAE3", display: "flex", alignItems: "center", gap: 10, padding: "0 18px" }}>
          {["#E5604D", "#E4B03C", "#58B864"].map((c) => <div key={c} style={{ width: 13, height: 13, borderRadius: 7, background: c }} />)}
          <div style={{ marginLeft: 18, flex: 1, height: 34, borderRadius: 17, background: "#fff", display: "flex", alignItems: "center", gap: 10, padding: "0 16px", fontFamily: SANS, fontSize: 21, color: C.ink }}>
            <svg width="16" height="18" viewBox="0 0 16 18"><rect x="1.5" y="7.5" width="13" height="9.5" rx="2" fill="#3C8D4E" /><path d="M4.5 7.5V5a3.5 3.5 0 0 1 7 0v2.5" stroke="#3C8D4E" strokeWidth="2" fill="none" /></svg>
            <span><span style={{ color: C.inkSoft }}>https://</span>{host}</span>
          </div>
        </div>
        <div style={{ width: W, height: H, position: "relative", overflow: "hidden" }}>
          <div style={{ width: 1920, height: 1080, transform: `scale(${W / 1920})`, transformOrigin: "0 0" }}>{children}</div>
        </div>
      </div>
    </AbsoluteFill>
  );
};

const Box: React.FC<{ x: number; y: number; w: number; h: number; label: string; sub?: string; color: string; show: number }> = ({ x, y, w, h, label, sub, color, show }) => (
  <div style={{ position: "absolute", left: x, top: y, width: w, height: h, borderRadius: 18, border: `3px solid ${color}`, background: "rgba(255,255,255,.04)",
    opacity: show, transform: `translateY(${(1 - show) * 18}px)`, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 6 }}>
    <div style={{ fontFamily: SANS, fontWeight: 700, fontSize: 36, color: C.onDark }}>{label}</div>
    {sub && <div style={{ fontFamily: SANS, fontSize: 24, color: C.onDarkSoft }}>{sub}</div>}
  </div>
);

/** E02 2.2 — your box → Docker → TroubaCore + Caddy → the internet; the data volume. Revealed with the voice. */
export const StackDiagram: React.FC<{ segs: [number, number][] }> = ({ segs }) => {
  const f = useCurrentFrame();
  const { fps } = useVideoConfig();
  const at = (i: number, extra = 0) => spring({ frame: f - Math.round(((segs[i]?.[0] ?? 0) + extra) * fps), fps, config: { damping: 200 } });
  const s0 = at(0), s1 = at(1), s2 = at(1, 2.2), s3 = at(2), s4 = at(2, 1.6);
  return (
    <DarkGround>
      <Box x={150} y={300} w={1320} h={520} label="" color={C.onDarkSoft} show={s0} />
      <div style={{ position: "absolute", left: 180, top: 316, fontFamily: MONO, fontSize: 26, color: C.onDarkSoft, opacity: s0 }}>YOUR SERVER · Docker</div>
      <Box x={240} y={400} w={480} h={180} label="TroubaCore" sub="one program" color={C.core} show={s1} />
      <Box x={240} y={620} w={480} h={150} label="/data" sub="one data folder" color={C.stage} show={s2} />
      <Box x={880} y={400} w={480} h={180} label="Caddy" sub="HTTPS, automatic" color={C.studio} show={s3} />
      <div style={{ position: "absolute", left: 720, top: 486, width: 160, height: 4, background: C.onDarkSoft, opacity: s3 }} />
      <div style={{ position: "absolute", left: 1360, top: 486, width: 170, height: 4, background: C.onDarkSoft, opacity: s4 }} />
      <Box x={1530} y={400} w={260} h={180} label="🔒" sub="the internet" color={C.onDarkSoft} show={s4} />
    </DarkGround>
  );
};

/** A list revealed one item per narration sentence (the 2.3 checklist, the 2.7 data-volume card). */
export const RevealList: React.FC<{ heading: string; items: { text: string; seg: number }[]; segs: [number, number][]; arc: string }> = ({ heading, items, segs, arc }) => {
  const f = useCurrentFrame();
  const { fps } = useVideoConfig();
  return (
    <DarkGround>
      <AbsoluteFill style={{ justifyContent: "center", paddingLeft: 200 }}>
        <div style={{ fontFamily: SERIF, fontWeight: 700, fontSize: 70, color: C.onDark, marginBottom: 46 }}>{heading}</div>
        {items.map((it, i) => {
          const s = spring({ frame: f - Math.round((segs[it.seg]?.[0] ?? 0) * fps), fps, config: { damping: 200 } });
          return (
            <div key={i} style={{ display: "flex", alignItems: "center", gap: 28, marginBottom: 30, opacity: s, transform: `translateX(${(1 - s) * -30}px)` }}>
              <div style={{ width: 46, height: 46, borderRadius: 23, background: ARC[arc], color: "#0F151B", fontFamily: SANS, fontWeight: 800, fontSize: 28,
                display: "flex", alignItems: "center", justifyContent: "center" }}>{i + 1}</div>
              <div style={{ fontFamily: SANS, fontSize: 44, color: C.onDark }}>{it.text}</div>
            </div>
          );
        })}
      </AbsoluteFill>
    </DarkGround>
  );
};
