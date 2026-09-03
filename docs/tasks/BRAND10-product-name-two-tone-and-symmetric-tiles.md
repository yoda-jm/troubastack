# BRAND10 — two-tone product names everywhere, symmetric Home tiles, and a state signal you can actually see

**Lane:** mobile (plus one web check). **Size:** S. **Status:** spec, not started.
**Raised by:** VLL, 2026-09-03, on the landed [BRAND09](BRAND09-home-wears-the-product-colours.md)
(`72735804`): *"in both mobile and web, the whole TroubaXXX is of the color of XXX, I think only Stage,
Studio should be according to brand, also on mobile TroubaStage background on homepage is default
background, whereas studio is light pink, rule for one of them."*

BRAND09's machinery is right — theme-aware tokens, no raw hex at call sites, a derived idle. This task
fixes what it does with them. Third finding below is mine, measured while checking his two.

## 1. The product name is two-tone — and the two platforms already disagree

`HomeScreen.kt` colours the **whole** string:

```kotlin
Text("▶  TroubaStage", …, color = accents.stage)
```

The web has already settled this the other way: `c330b5aa` made the masthead *"two-tone like the
wordmark — 'Trouba' ink, only 'Studio' accent"*, on VLL's ruling that **only the product suffix carries
the brand colour**. The wordmark itself is built that way — BRAND06 stores `TROUBA` once as a shared
base and a per-mark **accent tail**.

**So the app is not merely a matter of taste — it contradicts both the web and the wordmark.**

**Rule, to hold everywhere:** `Trouba` renders in ink (`onSurface`); only the product suffix
(`Stage`, `Studio`, `Core`) takes the product accent. Applies to both Home tiles, and to any other
place a product name is rendered. **Also re-check the web** for any remaining whole-name accent — the
masthead is fixed, other surfaces were not audited.

## 2. The two tiles use two different treatments — pick the filled one

- **Stage:** accent **border** on the default surface (`border = BorderStroke(1.5.dp, accents.stage)`).
- **Studio:** accent **fill** (`containerColor = accents.studioActive` = `#F8E4F0`, the "light pink").

The asymmetry is not arbitrary — Studio's fill carries its enabled/disabled state, and Stage is always
enabled (offline perform, I12), so it has no state to express. But it reads as arbitrary, and VLL asked
for one rule.

**Give Stage a fill too**, in its own family, and keep the border. Dropping Studio's fill instead would
regress the connected/disabled signal BRAND09 was asked to add.

Measured candidates, so this is not a guess (accent-on-tint needs ≥3:1 for a heading, body text ≥4.5:1
against the tint):

| tint | accent on it | `onSurface` text on it |
|---|---|---|
| Studio light `#F8E4F0` (shipped) | 3.81 ✔ | 14.15 ✔ |
| **Stage light `#F7EEDC`** | **4.18 ✔** | **14.86 ✔** |
| Studio dark `#2E1823` (shipped) | 3.59 ✔ | 12.80 ✔ |
| **Stage dark `#2A2113`** | **5.69 ✔** | **12.27 ✔** |

Add `stageActive` to `BrandAccents` alongside `studioActive`. Stage needs no idle variant.

## 3. ⚠ The connected/disabled signal is measurably too subtle to work

This is the one VLL did not raise, and it defeats the point of the change he *did* ask for.

| | ΔE between the two fills |
|---|---|
| light: active `#F8E4F0` vs idle `#EDE7EA` | **6.83** |
| dark: active `#2E1823` vs idle `#221C20` | **9.23** |
| (reference) surface vs active fill | 12.20 |

ΔE ≈ 2.3 is *just perceptible side by side*; ≈10 reads as obviously different without a reference.
**And a user never sees the two states side by side** — one tile is on screen and they must judge from
memory. At ΔE 6.8 that is not a signal, it is a coincidence. The tile does read as *tinted* versus the
page (12.20), so what fails is precisely the state distinction.

**Push the two states apart.** Options, cheapest first: desaturate `studioIdle` much further toward a
true neutral so "grey ⇒ disabled" is literal; or let the disabled state drop the fill entirely and
fall back to `surfaceVariant` (BRAND09's derivation intent survives — the *enabled* fill is branded);
or keep the fill and add a second cue. **Target ΔE ≥ 12 between the two states**, matching the
tint-vs-surface separation that already reads clearly.

Whatever is chosen, A55's behaviour is untouched: the tile is still `enabled = false` with its reason.

## Done when

- No product name renders wholly in its accent — `Trouba` is ink, the suffix is accent — on **mobile
  and web**, verified by reading the rendered output, not the token table.
- Both Home tiles use the **same treatment**, each in its own product colour.
- The connected and disabled Studio tiles differ by **ΔE ≥ 12**, asserted in a unit test over the
  token table for both grounds. Include the shipped pair (6.83) as a case that must **fail** the
  assertion — otherwise the test does not guard the thing it exists for.
- Accent-on-tint ≥ 3:1 for headings and body text ≥ 4.5:1 on every tile fill, both grounds.
- A55's enablement behaviour unchanged; `:shared:testDebugUnitTest` green, count matched; device-checked
  in both themes, connected and disconnected.

## Sequencing

VLL raised it on shipped behaviour he is looking at now, but it is polish, not a defect on the stand —
so **after the gig**, unless he says otherwise.
