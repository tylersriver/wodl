import SwiftUI

@main
struct WodlApp: App {
    @StateObject private var appState = AppState()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(appState)
        }
    }
}

class AppState: ObservableObject {
    @Published var isAuthenticated = false
    @Published var biometricEnabled = false

    private let keychain = KeychainService()

    init() {
        biometricEnabled = keychain.hasDeviceToken()
    }
}
