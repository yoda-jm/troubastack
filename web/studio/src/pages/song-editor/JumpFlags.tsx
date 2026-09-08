/**
 * P206 ⟨D2⟩ — the red mark on a jump whose pair cannot resolve within this file (the ones the bake drops:
 * a destination deleted, on another part, or the mark pointing at itself).
 *
 * Its own component ON PURPOSE (Fable, ⟨GO⟩ 4957749c). Studio can no longer AUTHOR a broken jump — the
 * delete takes the pair and an abandoned chain removes its landmark — so nothing reachable through the UI
 * can make this draw, and an e2e cannot get near it. The state still arrives: from an import, from an older
 * song, from another server. "A rule that makes a state unauthorable makes its handler need testing MORE",
 * so the seam moves down a level, from the surface to the state — this renders from data handed in, and
 * jump-flag.dom.test.tsx hands it the broken jump directly.
 */
import { objectBBox, type TextMeasure } from "../../editor";
import type { AnnotationObject } from "../../api";

export function JumpFlags({
  objects,
  broken,
  measure,
}: {
  /** The objects on THIS page, in paint order. */
  objects: AnnotationObject[];
  /** Uuids of the jump sources whose pair cannot resolve (brokenJumpUuids, over this FILE's objects). */
  broken: ReadonlySet<string>;
  /** Page box for text measurement; undefined before the page is measured. */
  measure?: TextMeasure;
}) {
  return (
    <>
      {objects
        .filter((o) => broken.has(o.uuid))
        .map((o) => {
          const b = objectBBox(o, measure);
          return (
            <div
              key={`broken-${o.uuid}`}
              className="jump-broken"
              data-testid="jump-broken"
              data-uuid={o.uuid}
              style={{
                left: `${b.minX * 100}%`,
                top: `${b.minY * 100}%`,
                width: `${(b.maxX - b.minX) * 100}%`,
                height: `${(b.maxY - b.minY) * 100}%`,
              }}
            />
          );
        })}
    </>
  );
}
