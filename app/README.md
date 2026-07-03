# TroubaShare — the mobile app

TroubaShare is the **shipped client**: a Kotlin Multiplatform / Compose Multiplatform
(KMP/CMP) application. **Android now, iOS later** (CMP on iOS is stable as of 2025).

It does exactly two jobs on a device:

1. **Hosts TroubaStudio** — the canonical web editor — inside a platform WebView.
   The editor is **never reimplemented natively** (**I10**). The app embeds the *built*
   Studio SPA; it does not import Studio source (**I14**).
2. **Performs TroubaStage** — the offline presenter. A baked concert is just **flattened
   images** (per page: a PDF raster + transparent annotation overlays), and the presenter
   is a **pure image compositor + pager** with **no** annotation-model or access-control
   logic (**I12**).

See the constitution: [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md).

> **Status (through A05).** The build is wired (committed wrapper; needs a JDK + Android SDK via
> `ANDROID_HOME` or `app/local.properties`): `cd app && ./gradlew :shared:check :androidApp:assembleDebug`,
> or `make app` from the repo root. **Working today:** the TroubaStage presenter (offline, resilient,
> read-only — A04), the bundle model/loader (A02) and atomic `.tstage` import via the Storage seam
> (A05). To try it on a device with zero servers, see the root README's "The mobile app" section and
> the committed demo bundle `docs/demo/demo-concert.tstage`. **Still `TODO()`:** the WebViewHost seam
> (Studio in the app — A06), the native ink overlay (A07, blocked on the tablet spike), the sync
> client and the downloader/updates. iOS stays "later": the target is commented out, its actuals are stubs.

---

## Why KMP/CMP — and the shared-vs-native split

> Toolchains (from [`../docs/design/06-tech-stack.md`](../docs/design/06-tech-stack.md)):
> JDK 25+, Kotlin Multiplatform.

The whole point of the app layer is to be a **thin shell**. Almost all of its behaviour is
device-agnostic, so it lives **once** in shared Kotlin (`shared/src/commonMain`). The
**shared module *is* the "mobile library"** referenced in the tech-stack doc.

**Shared (commonMain) — everything that *could* be shared, is:**

| Area | Lives in | Notes |
|---|---|---|
| TroubaStage presenter (image compositor + pager) | `shared/.../stage/` | dumb, offline, self-contained (**I12**) |
| Downloader / distribution / revision & availability logic | `shared/.../distribution/` | "what's available to me", update policy, atomic swap (**I13**) |
| Optimistic sync client + outbox | `shared/.../sync/` | talks to TroubaCore over WebSocket (**I6**) |
| Navigation, screen state, app glue | `shared/` (added later) | Compose Multiplatform UI |

**Native — limited to EXACTLY THREE `expect`/`actual` seams (`I15`):**

| # | Seam | `expect` | Android `actual` | iOS `actual` |
|---|---|---|---|---|
| 1 | **WebView host** — hosts the Studio SPA (**I10**) | `seams/WebViewHost.kt` | `android.webkit.WebView` | `WKWebView` |
| 2 | **Low-latency ink overlay** — wet freehand only (**I9**) | `seams/InkOverlay.kt` | Jetpack Ink (`androidx.ink`) / `GLFrontBufferedRenderer` | PencilKit / Metal |
| 3 | **Storage** — local paths / secure prefs | `seams/Storage.kt` | `Context.filesDir` / `EncryptedSharedPreferences` | `FileManager` / Keychain |

### The rule (I15)

> **If it could be shared Kotlin, or it could live in the webview, it must NOT be native.**

Reviewer test (from [`../docs/design/07-boundaries-and-no-duplication.md`](../docs/design/07-boundaries-and-no-duplication.md)):
*"Could this have been shared Kotlin or lived in the webview?"* If yes, it does not belong
in `androidMain`/`iosMain`. Anything native beyond these three seams is a smell. **iOS = fill
in the three `actual`s** — nothing else should need a second implementation.

### The one sanctioned duplication (I8)

Seam 2 (the ink overlay) is the **only** permitted re-implementation of stroke rendering.
The single stroke renderer is `web/ink`; the native overlay must match it **pixel-for-pixel**
and that parity is golden-image-tested. The overlay renders **only the in-progress (wet)
freehand stroke** (**I9**); on commit the stroke migrates native → web and the native layer
clears. There is **no third copy** — everything dry, and every other tool, renders in the web
layer (**I8**, **I10**).

---

## Layout

```
app/
├── README.md                 ← you are here
├── settings.gradle.kts       ← module graph: include(shared, androidApp); iosApp commented (iOS-later)
├── build.gradle.kts          ← root build: declares the KMP/CMP/AGP plugins (apply false)
├── gradle.properties         ← Gradle/AndroidX flags
├── gradle/libs.versions.toml ← version catalog: pinned tool + library versions
├── gradlew / gradle/wrapper/ ← committed Gradle wrapper (reproducible builds)
├── shared/                   ← the "mobile library": all shared Kotlin
│   └── src/
│       ├── commonMain/kotlin/
│       │   ├── gen/                              ← generated proto types land here (I1, git-ignored)
│       │   └── com/troubashare/shared/
│       │       ├── seams/                        ← the THREE expect seams (I15)
│       │       │   ├── WebViewHost.kt            ← seam 1 (I10)
│       │       │   ├── InkOverlay.kt             ← seam 2 (I9, I8)
│       │       │   └── Storage.kt                ← seam 3
│       │       ├── stage/                        ← shared presenter: StageModel/ViewModel/Screen (I12)
│       │       ├── bundle/                       ← bundle model + loader + atomic importer (A02/A05)
│       │       ├── distribution/Updates.kt       ← shared downloader / revisions (I13)
│       │       └── sync/SyncClient.kt            ← shared optimistic client (I6)
│       ├── androidMain/kotlin/com/troubashare/shared/seams/   ← the three Android actuals
│       └── iosMain/kotlin/com/troubashare/shared/seams/       ← the three iOS actuals (TODO, iOS-later)
├── androidApp/               ← thin Android entrypoint: concerts list, Stage host, import wiring
└── iosApp/                   ← thin iOS entrypoint (iOS-later; not yet a Gradle module)
```

## On generated types (I1)

Every domain type and wire message is defined **once** in [`../proto/`](../proto) and the
Kotlin bindings are **generated** into `shared/src/commonMain/kotlin/gen/` (see
[`../proto/buf.gen.yaml`](../proto/buf.gen.yaml)). That directory is git-ignored and
regenerated in CI — **never hand-edited**. Shared code references those generated types
(e.g. `ConcertBundle`, `Mutation`, `AvailableConcerts`); the stubs here use placeholder
aliases until codegen is wired up.
