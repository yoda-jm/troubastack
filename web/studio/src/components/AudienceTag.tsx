/**
 * T55 — the global-vs-personal audience tag (scheme A's one vocabulary component):
 * 👥 Band (shared with everyone) vs 👤 Mine (just for you). Reused wherever a surface
 * needs to declare its audience — the "Drawing on:" indicator now, the layer drawer /
 * "Bake my parts" in the later sweep. See docs/design/09-global-vs-personal-ia.md.
 */
import type { JSX } from "react";

export type Audience = "band" | "mine";

/** Classify a layer's ZONE by who sees the effect (Fable's rule): a personal layer is
 *  Mine; shared + conductor layers are Band (conductor = restricted authorship, not a
 *  third audience). */
export function audienceForZone(zone: string): Audience {
  return zone === "personal" ? "mine" : "band";
}

export function AudienceTag({
  audience,
  label,
  note,
}: {
  audience: Audience;
  /** Override the default short word ("Band" / "Mine"). */
  label?: string;
  /** Optional trailing note, e.g. "conductor". */
  note?: string;
}): JSX.Element {
  const band = audience === "band";
  return (
    <span className={`audience-tag audience-${audience}`} data-testid="audience-tag" data-audience={audience}>
      <span aria-hidden="true">{band ? "👥" : "👤"}</span> {label ?? (band ? "Band" : "Mine")}
      {note ? <span className="audience-note"> ({note})</span> : null}
    </span>
  );
}
