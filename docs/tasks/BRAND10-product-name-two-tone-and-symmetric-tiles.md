# BRAND10 — two-tone product names everywhere, symmetric Home tiles, and a state signal you can actually see

**Lane:** mobile (plus one web check). **Size:** S. **Status:** point 1 **LANDED** (`87e1460b`, GO);
points 2-3 **ruled by VLL 2026-09-03** and rewritten below.
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

## 2. The two tiles use two different treatments — OUTLINE for both (ruled)

- **Stage:** accent **border** on the default surface (`border = BorderStroke(1.5.dp, accents.stage)`).
- **Studio:** accent **fill** (`containerColor = accents.studioActive` = `#F8E4F0`, the "light pink").

The asymmetry is not arbitrary — Studio's fill carries its enabled/disabled state, and Stage is always
enabled (offline perform, I12), so it has no state to express. But it reads as arbitrary, and VLL asked
for one rule.

**RULED — VLL chose the OUTLINE for both.** So Studio **loses** its fill and matches Stage: default
surface, accent border. The measured fill candidates below are kept only as the record of what was
weighed; **do not implement them**.

⚠ **Verified on `origin/main` (`5be8cbd3`): this is NOT yet in the code.** Stage is outlined
(`HomeScreen.kt:462-463`) but Studio is still filled (`containerColor = accents.studioActive`,
`disabledContainerColor = accents.studioIdle`, `:515-516`). The ruling still has to be implemented.

Measured candidates, so this is not a guess (accent-on-tint needs ≥3:1 for a heading, body text ≥4.5:1
against the tint):

| tint | accent on it | `onSurface` text on it |
|---|---|---|
| Studio light `#F8E4F0` (shipped) | 3.81 ✔ | 14.15 ✔ |
| **Stage light `#F7EEDC`** | **4.18 ✔** | **14.86 ✔** |
| Studio dark `#2E1823` (shipped) | 3.59 ✔ | 12.80 ✔ |
| **Stage dark `#2A2113`** | **5.69 ✔** | **12.27 ✔** |

~~Add `stageActive`~~ — **not to be implemented.** The outline ruling means no new fill token is
needed, and `studioActive`/`studioIdle` are to be **removed**.

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

**RESOLVED by the outline ruling — the BORDER carries the state.** With no fill left, the connected
signal moves to the border: **accent border when connected, neutral `outlineVariant` when disabled.**
Measured against the theme's real values (`outlineVariant` `#E7E1D8` light / `#2B2836` dark):

| | ΔE, connected vs disabled border |
|---|---|
| Studio, light `#D62A8A` vs `#E7E1D8` | **82.73** |
| Studio, dark `#D62A8A` vs `#2B2836` | **73.31** |
| *(today's fills, for comparison)* | *6.83 light / 9.23 dark* |

**Roughly twelve times the separation**, in the idiom VLL chose — the ruling that made the tiles
consistent also fixed the signal that could not be seen. `studioActive` and `studioIdle` become
**unused and should be deleted**; keeping dead tokens invites someone to reintroduce the fill.

Stage is always enabled, so it always wears its accent border — no neutral variant needed.

Whatever is chosen, A55's behaviour is untouched: the tile is still `enabled = false` with its reason.

## Done when

- No product name renders wholly in its accent — `Trouba` is ink, the suffix is accent — on **mobile
  and web**, verified by reading the rendered output, not the token table.
- **Both Home tiles are outlined** on the default surface, each with its own product accent border —
  no fill on either.
- **The border carries the connected state**: accent when connected, `outlineVariant` when disabled,
  asserted in a unit test over the token table for **both grounds** (ΔE ≥ 12; the values are 82.73 and
  73.31, so there is ample margin). Include today's fill pair (**6.83**) as a case that must **fail**
  the assertion — a test that passes everything guards nothing.
- **`studioActive` and `studioIdle` are gone**, not merely unused — a dead fill token invites someone
  to bring the fill back.
- A55's enablement behaviour unchanged; `:shared:testDebugUnitTest` green, count matched; device-checked
  in both themes, connected and disconnected.

## Sequencing

**After the gig.** VLL raised it on shipped behaviour he is looking at now, but it is polish, not a
defect on the stand. Point 1 already landed (`87e1460b`); points 2-3 are one change — removing the fill
and moving the state onto the border — and should land together, since removing the fill *without*
moving the state would delete the connected signal entirely.
