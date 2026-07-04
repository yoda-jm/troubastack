import SwiftUI

// The thin iOS entrypoint (I15). Holds no logic beyond mounting the shared Compose UI — everything
// (Concerts list, Stage) lives in `:shared`'s `MainViewController`. Mirrors :androidApp's MainActivity.
@main
struct iOSApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
                .ignoresSafeArea(.all)          // Compose draws its own insets (safeContentPadding)
                .preferredColorScheme(.light)
        }
    }
}
