import Foundation
import Security
import LocalAuthentication

struct KeychainService {
    private let service = "app.wodl.device-token"
    private let account = "biometric-device-token"

    func saveDeviceToken(_ token: String) {
        // Delete any existing token first
        deleteDeviceToken()

        guard let data = token.data(using: .utf8) else { return }

        // Require biometric authentication to access this item
        let access = SecAccessControlCreateWithFlags(
            nil,
            kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly,
            .biometryCurrentSet,
            nil
        )

        var query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: data,
        ]

        if let access = access {
            query[kSecAttrAccessControl as String] = access
        }

        SecItemAdd(query as CFDictionary, nil)
    }

    func getDeviceToken() -> String? {
        let context = LAContext()
        context.localizedReason = "Access WODL login credentials"

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecUseAuthenticationContext as String: context,
        ]

        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)

        guard status == errSecSuccess, let data = result as? Data else {
            return nil
        }

        return String(data: data, encoding: .utf8)
    }

    func hasDeviceToken() -> Bool {
        // Check existence without triggering biometric prompt
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecUseAuthenticationUI as String: kSecUseAuthenticationUIFail,
        ]

        let status = SecItemCopyMatching(query as CFDictionary, nil)
        // errSecInteractionNotAllowed means item exists but needs biometric
        return status == errSecSuccess || status == errSecInteractionNotAllowed
    }

    func deleteDeviceToken() {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]

        SecItemDelete(query as CFDictionary)
    }
}
