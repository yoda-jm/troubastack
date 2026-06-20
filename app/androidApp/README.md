# androidApp — thin Android entrypoint

The thin Android shell (Activity + Compose Multiplatform host) that mounts the shared
`:shared` module. Holds **no** logic of its own beyond wiring the three Android `actual` seams
(WebView, `androidx.ink` overlay, storage) into the shared app (I15). Derived later — this is a
structural placeholder, not a build.
