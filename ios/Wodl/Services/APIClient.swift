import Foundation

enum APIError: Error {
    case unauthorized
    case badRequest
    case serverError(Int)
    case networkError(Error)
    case decodingError
}

struct APIClient {
    private let baseURL = URL(string: "https://wodl.up.railway.app")!

    /// Registers this device and returns the raw device token to store in the Keychain.
    func registerDevice(deviceName: String) async throws -> String {
        let url = baseURL.appendingPathComponent("/api/device-token")
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        let body = ["device_name": deviceName]
        request.httpBody = try JSONSerialization.data(withJSONObject: body)

        // Use shared URLSession which includes cookies from the WebView's cookie store
        let (data, response) = try await URLSession.shared.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw APIError.networkError(URLError(.badServerResponse))
        }

        switch httpResponse.statusCode {
        case 200...299:
            guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                  let token = json["device_token"] as? String else {
                throw APIError.decodingError
            }
            return token
        case 401:
            throw APIError.unauthorized
        default:
            throw APIError.serverError(httpResponse.statusCode)
        }
    }

    /// Exchanges a device token for a JWT session (sets the cookie server-side).
    func biometricLogin(deviceToken: String) async throws {
        let url = baseURL.appendingPathComponent("/api/biometric-login")
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        let body = ["device_token": deviceToken]
        request.httpBody = try JSONSerialization.data(withJSONObject: body)

        let (_, response) = try await URLSession.shared.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw APIError.networkError(URLError(.badServerResponse))
        }

        switch httpResponse.statusCode {
        case 200...299:
            // Sync cookies from URLSession to WKWebView's cookie store
            if let headerFields = httpResponse.allHeaderFields as? [String: String],
               let url = httpResponse.url {
                let cookies = HTTPCookie.cookies(withResponseHeaderFields: headerFields, for: url)
                for cookie in cookies {
                    await MainActor.run {
                        HTTPCookieStorage.shared.setCookie(cookie)
                    }
                }
            }
        case 401:
            throw APIError.unauthorized
        default:
            throw APIError.serverError(httpResponse.statusCode)
        }
    }
}
