import SwiftUI

struct ContentView: View {
    @EnvironmentObject var appState: AppState
    @StateObject private var viewModel = ContentViewModel()

    var body: some View {
        ZStack {
            WebView(
                url: viewModel.baseURL,
                isLoggedIn: $viewModel.isLoggedIn,
                onLoginDetected: {
                    viewModel.onLoginDetected()
                }
            )
            .ignoresSafeArea(edges: .bottom)

            // Biometric prompt overlay
            if viewModel.showBiometricSetup {
                biometricSetupOverlay
            }
        }
        .onAppear {
            viewModel.appState = appState
            viewModel.attemptBiometricLogin()
        }
    }

    private var biometricSetupOverlay: some View {
        VStack(spacing: 16) {
            Spacer()

            VStack(spacing: 12) {
                Image(systemName: BiometricService.shared.biometricIconName)
                    .font(.system(size: 40))
                    .foregroundColor(.blue)

                Text("Enable \(BiometricService.shared.biometricTypeName)?")
                    .font(.headline)

                Text("Use \(BiometricService.shared.biometricTypeName) to log in instantly next time.")
                    .font(.subheadline)
                    .foregroundColor(.secondary)
                    .multilineTextAlignment(.center)

                HStack(spacing: 12) {
                    Button("Not Now") {
                        viewModel.showBiometricSetup = false
                    }
                    .buttonStyle(.bordered)

                    Button("Enable") {
                        viewModel.enableBiometric()
                    }
                    .buttonStyle(.borderedProminent)
                }
                .padding(.top, 8)
            }
            .padding(24)
            .background(.regularMaterial)
            .cornerRadius(16)
            .shadow(radius: 10)
            .padding(.horizontal, 32)

            Spacer()
                .frame(height: 60)
        }
    }
}

@MainActor
class ContentViewModel: ObservableObject {
    let baseURL = URL(string: "https://wodl.up.railway.app")!

    @Published var isLoggedIn = false
    @Published var showBiometricSetup = false

    var appState: AppState?

    private let keychain = KeychainService()
    private let biometric = BiometricService.shared
    private let apiClient = APIClient()
    private var hasPromptedSetup = false

    func attemptBiometricLogin() {
        guard keychain.hasDeviceToken() else { return }

        Task {
            do {
                let authenticated = try await biometric.authenticate(
                    reason: "Log in to WODL"
                )
                guard authenticated else { return }

                guard let deviceToken = keychain.getDeviceToken() else { return }

                try await apiClient.biometricLogin(deviceToken: deviceToken)
                isLoggedIn = true
                appState?.isAuthenticated = true
            } catch {
                // Biometric failed or token invalid — user will see the login page
                if case APIError.unauthorized = error {
                    keychain.deleteDeviceToken()
                    appState?.biometricEnabled = false
                }
            }
        }
    }

    func onLoginDetected() {
        guard !hasPromptedSetup,
              !keychain.hasDeviceToken(),
              biometric.canUseBiometrics else { return }

        hasPromptedSetup = true
        showBiometricSetup = true
    }

    func enableBiometric() {
        showBiometricSetup = false

        Task {
            do {
                let deviceToken = try await apiClient.registerDevice(
                    deviceName: UIDevice.current.name
                )
                keychain.saveDeviceToken(deviceToken)
                appState?.biometricEnabled = true
            } catch {
                // Silently fail — user can still use the web login
            }
        }
    }
}
