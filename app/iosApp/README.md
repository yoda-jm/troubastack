# iosApp — thin iOS entrypoint

The thin iOS shell (SwiftUI / UIViewController hosting Compose Multiplatform) that mounts the
shared `:shared` module. ⚠️ iOS-LATER. Holds **no** logic beyond wiring the three iOS `actual`
seams (WKWebView, PencilKit/Metal overlay, storage) into the shared app (I15). Derived later —
structural placeholder, not a build.
