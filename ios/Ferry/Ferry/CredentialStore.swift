import Foundation
import Security

protocol CredentialStoring {
    func token(for origin: String) throws -> String?
    func save(token: String, for origin: String) throws
    func removeToken(for origin: String) throws
}

struct KeychainCredentialStore: CredentialStoring {
    private let service = "com.max1874.ferry.device"

    func token(for origin: String) throws -> String? {
        var query = baseQuery(origin)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess, let data = result as? Data, let token = String(data: data, encoding: .utf8) else {
            throw KeychainError(status)
        }
        return token
    }

    func save(token: String, for origin: String) throws {
        let data = Data(token.utf8)
        let query = baseQuery(origin)
        let update = [kSecValueData as String: data]
        let status = SecItemUpdate(query as CFDictionary, update as CFDictionary)
        if status == errSecItemNotFound {
            var item = query
            item[kSecValueData as String] = data
            let addStatus = SecItemAdd(item as CFDictionary, nil)
            guard addStatus == errSecSuccess else { throw KeychainError(addStatus) }
        } else if status != errSecSuccess {
            throw KeychainError(status)
        }
    }

    func removeToken(for origin: String) throws {
        let status = SecItemDelete(baseQuery(origin) as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else { throw KeychainError(status) }
    }

    private func baseQuery(_ origin: String) -> [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: origin]
    }
}

private struct KeychainError: LocalizedError {
    let status: OSStatus
    init(_ status: OSStatus) { self.status = status }
    var errorDescription: String? { "Could not access this device credential (Keychain error \(status))." }
}
