# androidApp — thin Android entrypoint

The thin Android shell (Activity + Compose Multiplatform host) that mounts the shared
`:shared` module. Holds **no** logic of its own beyond wiring the three Android `actual` seams
(WebView, `androidx.ink` overlay, storage) into the shared app (I15).

Wired in A01: `com.troubashare.app` with a `MainActivity` that shows a Compose placeholder
(the TroubaShare name + an inert "Stage" button). `./gradlew :androidApp:assembleDebug` produces
`androidApp/build/outputs/apk/debug/androidApp-debug.apk`. Later A-tasks replace the placeholder
with the real presenter (A04) and WebView-hosted Studio (A06) — both from `:shared`/behind seams.
