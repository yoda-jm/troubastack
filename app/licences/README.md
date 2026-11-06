# Open-source notices: the committed texts and tables (A101)

The apps' Licences screen (⚙ Parameters → About → Licences) is **generated at build time** by
`OpenSourceNotices` (`app/buildSrc`) from the dependencies each build resolves. Nothing here is a hand-written list
of components. What is committed is what the generator cannot take from the artifacts themselves:

- `spdx/` — licence texts, from SPDX's license-list-data v3.25.0 (`text/<id>.txt`): Apache-2.0, MIT, BSD-3-Clause,
  MPL-2.0. (EPL-2.0 is S2.8's `app/desktopApp/licences/EPL-2.0.txt`, shipped with SWT.)
- `licence-map.txt` — POM licence name/URL → SPDX id. An unmatched licence fails the build.
- `elections.txt` — a component offered under a choice of licences, and the one used (JNA: Apache-2.0).
- `extra-licences.txt` — a licence a component's shipped file names but its POM does not (OkHttp: MPL-2.0 for
  publicsuffixes.gz), pinned to the version checked.
- `delegated.txt` — components whose texts live in their own entry elsewhere (SWT, from S2.8).
- `families.txt` — which entry of the screen a component is filed under.
- `native/manifest.txt` + `native/*` — the third-party code inside native libraries, pinned to the version checked.

The generator reads every NOTICE and licence/copyright file inside each artifact verbatim, so those are not here.
No text is fetched at build time.

## Where each text came from (fetched 2026-11-06)

| File | Source |
|---|---|
| `native/skia.LICENSE` | github.com/google/skia@a00c390e98b1a8741f11774c742ff60d767365ac/LICENSE |
| `native/harfbuzz.COPYING` | github.com/harfbuzz/harfbuzz@a070f9ebbe88dc71b248af9731dd49ec93f4e6e6/COPYING |
| `native/icu.LICENSE` | https://chromium.googlesource.com/chromium/deps/icu.git@364118a1d9da24bb5b770ac3d762ac144d6da5a4/LICENSE |
| `native/libjpeg-turbo.LICENSE.md` | https://chromium.googlesource.com/chromium/deps/libjpeg_turbo.git@ccfbe1c82a3b6dbe8647ceb36a3f9ee711fba3cf/LICENSE.md |
| `native/libjpeg-turbo.README.ijg` | https://chromium.googlesource.com/chromium/deps/libjpeg_turbo.git@ccfbe1c82a3b6dbe8647ceb36a3f9ee711fba3cf/README.ijg |
| `native/libpng.LICENSE` | https://skia.googlesource.com/third_party/libpng.git@ed217e3e601d8e462f7fd1e04bed43ac42212429/LICENSE |
| `native/libwebp.COPYING` | https://chromium.googlesource.com/webm/libwebp.git@845d5476a866141ba35ac133f856fa62f0b7445f/COPYING |
| `native/libwebp.PATENTS` | https://chromium.googlesource.com/webm/libwebp.git@845d5476a866141ba35ac133f856fa62f0b7445f/PATENTS |
| `native/wuffs.LICENSE` | https://skia.googlesource.com/external/github.com/google/wuffs-mirror-release-c.git@e3f919ccfe3ef542cfc983a82146070258fb57f8/LICENSE |
| `native/zlib.LICENSE` | https://chromium.googlesource.com/chromium/src/third_party/zlib@646b7f569718921d7d4b5b8e22572ff6c76f2596/LICENSE |
| `native/expat.COPYING` | https://chromium.googlesource.com/external/github.com/libexpat/libexpat.git@624da0f593bb8d7e146b9f42b06d8e6c80d032a3/expat/COPYING |
| `native/dng_sdk.LICENSE` | https://android.googlesource.com/platform/external/dng_sdk.git@c8d0c9b1d16bfda56f15165d39e0ffa360a11123/LICENSE |
| `native/freetype.FTL.TXT` | https://chromium.googlesource.com/chromium/src/third_party/freetype2.git@83af801b552111e37d9466a887e1783a0fb5f196/docs/FTL.TXT |
| `native/piex.LICENSE` | https://android.googlesource.com/platform/external/piex.git@bb217acdca1cc0c16b704669dd6f91a1b509c406/LICENSE |
| `native/d3d12allocator.LICENSE.txt` | https://skia.googlesource.com/external/github.com/GPUOpen-LibrariesAndSDKs/D3D12MemoryAllocator.git@169895d529dfce00390a20e69c2f516066fe7a3b/LICENSE.txt |
| `native/jna-libffi.LICENSE` | github.com/java-native-access/jna@5.17.0/native/libffi/LICENSE |
| `native/libyuv.LICENSE` | https://chromium.googlesource.com/libyuv/libyuv@refs/heads/main/LICENSE |
| `spdx/Apache-2.0.txt` | github.com/spdx/license-list-data@v3.25.0/text/Apache-2.0.txt |
| `spdx/MIT.txt` | github.com/spdx/license-list-data@v3.25.0/text/MIT.txt |
| `spdx/BSD-3-Clause.txt` | github.com/spdx/license-list-data@v3.25.0/text/BSD-3-Clause.txt |
| `spdx/MPL-2.0.txt` | github.com/spdx/license-list-data@v3.25.0/text/MPL-2.0.txt |
| `native/freetype.CREDIT.txt` | written here: the FTL's required credit line, with the year of the FreeType revision above (2024-09-07), and the election of the FTL over GPLv2 |

## Skiko 0.9.22.2: what its native library bundles

Skiko's `skiko/gradle.properties` at v0.9.22.2 pins `dependencies.skia=m132-a00c390e98-1`: Skia
a00c390e98b1a8741f11774c742ff60d767365ac, built by JetBrains skia-pack's `script/build.py` at tag m132-a00c390e98-1.
That script sets `skia_use_system_{expat,libjpeg_turbo,libpng,libwebp,zlib,freetype2,harfbuzz,icu}=false` (bundled from
Skia's `third_party/externals`, at the revisions in Skia's DEPS) and `skia_use_direct3d=true` on Windows; everything else
is Skia's `gn/skia.gni` default. **The build configuration decides; `strings` on each OS's library is the cross-check**
(Windows' DLL keeps few symbol names, so it confirms only libraries that leave a version banner).

| Library | Decided by | Linux | Windows | macOS | `strings` saw it in |
|---|---|---|---|---|---|
| Skia | the library itself | ✓ | ✓ | ✓ | all |
| FreeType | skia.gni `skia_use_freetype = is_android \|\| is_fuchsia \|\| is_linux \|\| is_wasm` (bundled: skia-pack) | ✓ | – | – | Linux only (`FT_Init_FreeType`) |
| HarfBuzz | skia.gni `skia_use_harfbuzz = true` (bundled: skia-pack) | ✓ | ✓ | ✓ | Linux, macOS |
| ICU | skia.gni `skia_use_icu = !is_fuchsia` (bundled: skia-pack) | ✓ | ✓ | ✓ | Linux, macOS |
| libjpeg-turbo | skia.gni `skia_use_libjpeg_turbo_{decode,encode} = true` (bundled) | ✓ | ✓ | ✓ | all ("libjpeg-turbo version 2.1.5.1") |
| libpng | skia.gni `skia_use_libpng_{decode,encode} = true` (bundled) | ✓ | ✓ | ✓ | Linux, Windows ("libpng version 1.6.43") |
| libwebp | skia.gni `skia_use_libwebp_decode = true` (bundled) | ✓ | ✓ | ✓ | Linux, macOS |
| Wuffs | skia.gni `skia_use_wuffs = true` | ✓ | ✓ | ✓ | all |
| zlib | skia.gni `skia_use_zlib = true` (bundled: skia-pack) | ✓ | ✓ | ✓ | Linux, Windows ("inflate 1.3.0.1") |
| Expat | skia.gni `skia_use_expat = !is_wasm` (bundled) | ✓ | ✓ | ✓ | Linux, macOS |
| DNG SDK | skia.gni `skia_use_dng_sdk = !is_fuchsia && !is_wasm && libjpeg_turbo_decode && zlib` | ✓ | ✓ | ✓ | Linux, macOS |
| piex | skia.gni `skia_use_piex = !is_win && !is_wasm` | ✓ | – | ✓ | Linux, macOS |
| D3D12 Memory Allocator | skia-pack `skia_use_direct3d=true` (Windows); Skia `BUILD.gn` adds `//third_party/d3d12allocator` | – | ✓ | – | (none: Windows symbols stripped) |

Not bundled: Vulkan Memory Allocator (`skia_use_vulkan` is false off Fuchsia); Brotli (FreeType links it only for
WOFF2, `is_wasm`); fontconfig (Linux, a *system* library, linked not shipped); sfntly (off).

## Elections

- **JNA** (LGPL-2.1-or-later OR Apache-2.0): used under **Apache-2.0**.
- **FreeType** (FTL OR GPLv2): used under the **FTL**, with its credit line.

## The CI check (`ci/`)

`ci/check-notices.py` checks the notices inside each shipped artifact (the APK, the MSI's admin extract, the extracted
AppImage, the universal macOS app) without running the generator's code. Its expected components come from Gradle's
own `dependencies` report and a separate artifact lookup (`ci/artifact-paths.init.gradle.kts`); what ships is read from
the artifact. It prints `A101-CHECK coverage-…`, `notices-…` and `natives-…`, each with what it excluded or mapped. `notices-…`
recognises five licence names, by URL or by name (MPL-2.0, Apache-2.0, LGPL, EPL, MIT): a shipped file that names any
other licence is not caught; none does today (Fable scanned the generated texts, 2026-11-06).
