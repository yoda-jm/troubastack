# P205 acceptance kit — two identities, ONE file, different layers + cues

A ~10-minute attended run proving the P205 program end-to-end on a real device:
**one band-wide bundle, resolved to different views per identity.** No app code —
this is verification scaffolding any session can run when the QA tablet is back.

**The claim to prove:** load the single band-wide demo bundle, view it as **Marie**
(auto-matched when connected) then switch to **Sasha** ("not you?") — *different cues
flash, a different layer list, same file.*

---

## What to expect (from the shipped band-wide `docs/demo/demo-concert.tstage`)

Roster: **Marie** (admin) · **Leo** (conductor) · **Sasha** (member). On **Wonderwall**:

| Identity | Cue flash on entry | Layers dialog lists |
|---|---|---|
| **Marie** | 🎤 mic · 🎸 electric guitar (red `#e11d48`) | Section markings · Chords & capo · **Breath & phrasing** (Marie's) |
| **Sasha** | 🎸 bass (blue `#2563eb`) | Section markings · Chords & capo *(shared only — Sasha has no personal layer here)* |
| _(Leo, if picked)_ | 🎸 acoustic guitar | Section markings · Chords & capo · **Chords** · **Conductor cues** (Leo's) |

The **shared** layers (Section markings, Chords & capo) appear for everyone; each
member's **personal** layers appear only for them (Stage 3b filtering); the **cues**
come from that identity's `member_cues` (Stage 3a). Same `.tstage` throughout.

---

## Setup (one-time, ~5 min)

Everything below is host-side; paths assume the repo worktree + Android SDK on PATH
(`export PATH="$PATH:$HOME/Android/Sdk/platform-tools"`).

1. **Wireless adb** (survives the 100%-battery USB drop — see the session device-QA notes):
   ```sh
   adb devices                                   # USB visible?
   adb tcpip 5555
   adb connect <tablet-ip>:5555                  # ip: adb shell ip -f inet addr show wlan0
   ```
2. **Build + install current main:**
   ```sh
   ./app/gradlew -p app :androidApp:assembleDebug
   adb -s <ip>:5555 install -r app/androidApp/build/outputs/apk/debug/androidApp-debug.apk
   ```
3. **Load the ONE band-wide bundle.** Either:
   - **Normal (VLL's flow):** in-app Home → Concerts → Import → pick
     `docs/demo/demo-concert.tstage` via the file picker. *(Push it to the tablet
     first: `adb -s <ip>:5555 push docs/demo/demo-concert.tstage /sdcard/Download/`.)*
   - **Fast (any session, no picker):** unzip + `run-as` into the app's private dir
     (debug build). `concertId` is in `bundle.json`; sanitize non-`[A-Za-z0-9-_]`→`_`:
     ```sh
     rm -rf /tmp/bw && mkdir /tmp/bw && (cd /tmp/bw && unzip -q .../docs/demo/demo-concert.tstage)
     SEG=$(python3 -c "import json;print(''.join(c if c.isalnum() or c in '-_' else '_' for c in json.load(open('/tmp/bw/bundle.json'))['concertId']))")
     adb -s <ip>:5555 shell rm -rf /data/local/tmp/bw && adb -s <ip>:5555 push /tmp/bw /data/local/tmp/bw
     adb -s <ip>:5555 shell run-as com.troubashare.app sh -c "rm -rf files/bundles/$SEG && mkdir -p files/bundles/$SEG && cp -r /data/local/tmp/bw/. files/bundles/$SEG/"
     ```

---

## The run (~5 min)

Wake + launch: `adb -s <ip>:5555 shell input keyevent KEYCODE_WAKEUP; adb -s <ip>:5555 shell monkey -p com.troubashare.app -c android.intent.category.LAUNCHER 1`

1. **Home** → cold-start lands on the landing page. **A31:** two branded products —
   **TroubaStage** (perform) + **TroubaStudio** (author/manage; Concerts nests inside it) —
   plus a **live** identity line ("Checking…" → "Performing as <name> ✓" / "Offline …" /
   "Connect to your band"), probed on entry + resume, never a cached flag.
2. **Perform** (tap TroubaStage) → open **Sat @ The Anchor** → Wonderwall.
   - **Connected as Marie** → **auto-matched, no picker**; identity reads "Performing as Marie".
     *(This genuinely fires as of A31 — the pre-A31 `/api/me` wrapper bug left the userId empty,
     so the picker ALWAYS appeared even when logged in; auto-match now needs the bundle's roster
     to carry the connected member's server id, e.g. a bundle baked from that server.)* Anonymous
     (or a roster whose ids predate the server) → the **"Who are you?"** picker appears; pick **Marie**.
   - ✅ **Cue flash:** 🎤 mic + 🎸 red electric guitar (center, on entry).
   - Open ⚙ → **Layers…**: ✅ lists Section markings · Chords & capo · Breath & phrasing (NOT Chords / Conductor cues).
3. **Switch identity:** ⚙ → **"Performing as Marie · Switch"** → pick **Sasha**.
   - ✅ Re-enter Wonderwall (or observe on next entry): **cue flash = 🎸 blue bass**; **Layers… lists Section markings · Chords & capo only**.
4. ✅ **Same file** — you never re-imported; only the identity changed. Two identities → two views → one bundle. **Acceptance met.**

Pixels for the record: `adb -s <ip>:5555 shell screencap -p /sdcard/s.png && adb -s <ip>:5555 pull /sdcard/s.png`.

---

## adb gotchas (this session's hard-won map — see the session device-QA notes)

- **`input tap x y` uses LOGICAL screen coords, 1:1** (the coords you read off a screenshot). Don't rotate them.
- **Stage tap map (landscape, chrome up):** ⚙ settings ≈ `1737,62`; then **Layers…** ≈ `1312,1073`; ☰ drawer ≈ `70,64`; ‹ `98,1061` · › `1822,1061`; ✕ exit `1849,64`. Concerts screen is portrait (1:1).
- **Calibrate any tap** with the pointer overlay: `adb shell settings put system pointer_location 1` (off = `0`).
- **Screen off / PIN-locked** → `screencap` returns **black**; `KEYCODE_WAKEUP` + (if no PIN) swipe-up. A PIN keyguard can't be dismissed remotely — VLL unlocks.
- **USB drops at 100% battery** (re-enumerates to charge-only `ff08`) → the wireless adb above is the fix; also set Developer options → Default USB config = File Transfer + Sleep = long.
