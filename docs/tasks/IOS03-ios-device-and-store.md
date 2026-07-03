# IOS03 — iOS on real devices / App Store

**Priority:** iOS-track 3 · **Size:** — (decision stub)

## ⛔ BLOCKED — needs hardware + credentials no agent has

This is a decision stub (like A07), kept so nobody improvises. After IOS02 proves the
app on a simulator, going further requires things outside this repo:

| Goal | Needs | Cost |
|---|---|---|
| Run on **your own iPhone/iPad** | a Mac (or rented mac cloud) with Xcode + a **free** Apple ID (personal team) | free; app re-signs every 7 days, 3-app limit |
| TestFlight for the band | Apple Developer Program | $99/year |
| App Store release | Apple Developer Program + review | $99/year |

When unblocked, rewrite this as a real task. Contents to cover then: signing config
(kept out of the repo), `fastlane` or plain `xcodebuild -exportArchive`, the PencilKit
question only if A07 was ever built, and Stage-specific device QA (stand-mounted iPad,
guided access mode during performance).

Until then: Android is the shipped device story; iOS lives in CI simulators (IOS02).
