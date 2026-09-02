import SwiftUI
import Shared

// Hosts the Compose `UIViewController` exported by the `Shared` framework. `MainViewControllerKt` is
// the Kotlin-generated wrapper for the top-level `MainViewController()` in package
// com.troubastack.shared (file MainViewController.kt). No app logic here — pure glue (I15).
struct ContentView: UIViewControllerRepresentable {
    func makeUIViewController(context: Context) -> UIViewController {
        MainViewControllerKt.MainViewController()
    }

    func updateUIViewController(_ uiViewController: UIViewController, context: Context) {}
}
