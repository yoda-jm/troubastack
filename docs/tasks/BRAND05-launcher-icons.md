# BRAND05 — wire the launcher icons into the app

**Status:** **CODE LANDED** — 1 commit(s), `5e52896b`…`5e52896b` (last 2026-09-03). This line previously said "spec, not started"; it is a SECONDARY copy of a fact the review gate owns (`docs/handoff/reviews.md`), and it rotted. Corrected 2026-09-04 from the git history. **Not re-verified against this spec's own done-when** — "code landed" is what was checked, not "every criterion met".
**Asked by:** VLL — "oui spec aussi les icones du launcher".

## Current state, surveyed rather than assumed

**The app has no launcher icon at all.** There is no `mipmap` or `drawable` resource under
`app/androidApp/src/main/res`, and `AndroidManifest.xml` declares `android:label` but **no
`android:icon`**. So TroubaStage ships today wearing the stock Android silhouette — on a
tablet on a music stand, next to the band's other apps.

BRAND01 already generated what is needed, and it is committed:

```
docs/brand/dist/troubastage-adaptive-foreground.svg
docs/brand/dist/troubastage-adaptive-background.svg
```

**The app's mark is TroubaStage** — the play chip, layer 1 — not TroubaStack. TroubaStack is
the product; do not ship the Stack mark as the app icon.

## One fact that makes this much smaller than it looks

**`minSdk = 26`** (from `gradle/libs.versions.toml`). Adaptive icons landed in API 26, so
every device this app can run on supports them. That means:

- `res/mipmap-anydpi-v26/ic_launcher.xml` alone is sufficient;
- **no legacy PNG density ladder is needed** — no mdpi/hdpi/xhdpi/xxhdpi/xxxhdpi set;
- `android:roundIcon` is redundant: the launcher derives the round shape from the mask.

Do not generate the raster ladder "to be safe". It is five directories of files that no
supported device would ever read, and each one is a copy that can drift from the source.

## ⚠ The conversion trap — SVG → VectorDrawable is lossy, silently

The generated SVGs lean on features the VectorDrawable format handles differently:

1. **`gradientTransform` is not supported and is dropped without warning.** The foreground
   contains exactly one: `translate(515,640) scale(1,0.55) translate(-515,-640)` — the oval
   falloff on the staff rules' radial gradient (`gRule`). Converted naively, that gradient
   becomes **circular**, and the rules fade in a visibly different shape. Either bake the
   transform into the gradient's own coordinates before converting, or accept the change
   deliberately — but do not discover it on a device.
2. **`<clipPath>` becomes `<clip-path>`**, which is group-scoped and not antialiased the same
   way. Two are present.
3. **No `<filter>` anywhere** — BRAND01 already guarantees this, which is why the chip
   shadows, plane shadows and highlighter texture are built from plain shapes. Keep it that
   way; a filter added upstream would break this task silently.

**Required control arm:** render the converted VectorDrawable and the source SVG at the same
size and compare them side by side, by eye. A conversion that "succeeded" is not evidence;
the tooling reports success while dropping what it cannot express.

## The monochrome layer is authoring work, not a checkbox

Android 13+ themed icons need a `<monochrome>` layer: a **single-colour silhouette**, tinted
by the system. It cannot be derived automatically from a multicolour mark — flattening the
three planes and the chip to one colour yields a shape nobody can read. Someone has to draw
a silhouette that still says "Stage" at 48dp with no colour at all.

If that is not done, themed-icon users fall back to the standard icon. That is an acceptable
outcome — but it must be a **decision**, not an omission discovered later.

## Work

1. Convert both SVGs to VectorDrawables, honouring the trap above.
2. `res/mipmap-anydpi-v26/ic_launcher.xml` with `<background>`, `<foreground>`, and
   `<monochrome>` if the silhouette gets drawn.
3. `android:icon="@mipmap/ic_launcher"` in the manifest.
4. Decide and record: monochrome layer, yes or no.

## Verification — on the device, not in a preview

- Install and **look at the launcher** with a round mask, a squircle mask and a teardrop
  mask. BRAND01 fitted the art to the 66/108 safe circle (`FG_SCALE 0.5873`, measured via
  the art's minimal enclosing circle) and `build.py --png` re-verifies that no ink escapes —
  but that guarantees geometry, not that the conversion kept the paint.
- Check the icon in the app switcher and in Settings → Apps, where it renders small.
- **Device state goes stale — re-read it. Do not report from memory.**

## Timing against the concert

Purely cosmetic, so it is not gig-critical — but it is worth pairing with the reinstall
**BRAND02 already forces**: `applicationId` moved to `com.troubastack.app`, so every
performing device must be reinstalled before **2026-09-05** anyway. Landing this first means
one reinstall instead of two. Landing it late means shipping an untested visual change onto
the device that has to work on the night; if it is not ready in time, **do it after the
gig** — the stock icon has never stopped anyone playing.
