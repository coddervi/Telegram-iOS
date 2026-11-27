import Foundation
import Security

public struct PlatformUser: Codable, Sendable, Equatable {
    public let id: String
    public let username: String
    public let displayName: String
    public let role: String?

    enum CodingKeys: String, CodingKey {
        case id, username, role
        case displayName = "display_name"
    }
}

public struct AuthSession: Codable, Sendable {
    public let token: String
    public let expiresAt: String
    public let user: PlatformUser

    enum CodingKeys: String, CodingKey {
        case token, user
        case expiresAt = "expires_at"
    }
}

public struct PlatformConversation: Codable, Sendable, Identifiable {
    public let id: String
    public let kind: String
    public let title: String?
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, kind, title
        case createdAt = "created_at"
    }
}

public struct PlatformMessage: Codable, Sendable, Identifiable {
    public let id: String
    public let conversationID: String
    public let senderID: String
    public let body: String
    public let replyTo: String?
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, body
        case conversationID = "conversation_id"
        case senderID = "sender_id"
        case replyTo = "reply_to"
        case createdAt = "created_at"
    }
}

public struct WalletTransaction: Codable, Sendable, Identifiable {
    public let id: String
    public let amount: Int64
    public let balanceAfter: Int64
    public let reason: String
    public let note: String
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, amount, reason, note
        case balanceAfter = "balance_after"
        case createdAt = "created_at"
    }
}

public struct WalletSnapshot: Codable, Sendable {
    public let currency: String
    public let balance: Int64
    public let transactions: [WalletTransaction]
}

public struct PlatformGift: Codable, Sendable, Identifiable {
    public let id: String
    public let code: String
    public let title: String
    public let description: String
    public let rarity: String
    public let priceStars: Int64
    public let upgradeCostStars: Int64
    public let maxLevel: Int
    public let maxSupply: Int64?
    public let issuedCount: Int64

    enum CodingKeys: String, CodingKey {
        case id, code, title, description, rarity
        case priceStars = "price_stars"
        case upgradeCostStars = "upgrade_cost_stars"
        case maxLevel = "max_level"
        case maxSupply = "max_supply"
        case issuedCount = "issued_count"
    }
}

public struct GiftInventoryItem: Codable, Sendable, Identifiable {
    public let ownershipID: String
    public let giftID: String
    public let title: String
    public let rarity: String
    public let serialNumber: Int64
    public let level: Int
    public let acquiredAt: String

    public var id: String { ownershipID }

    enum CodingKeys: String, CodingKey {
        case title, rarity, level
        case ownershipID = "ownership_id"
        case giftID = "gift_id"
        case serialNumber = "serial_number"
        case acquiredAt = "acquired_at"
    }
}

public struct GiftPurchaseResult: Codable, Sendable {
    public let giftID: String
    public let serialNumber: Int64
    public let level: Int
    public let balance: Int64

    enum CodingKeys: String, CodingKey {
        case level, balance
        case giftID = "gift_id"
        case serialNumber = "serial_number"
    }
}

public enum PrivatePlatformError: Error, LocalizedError, Sendable {
    case invalidResponse
    case httpStatus(Int, String)
    case missingToken
    case invalidBaseURL

    public var errorDescription: String? {
        switch self {
        case .invalidResponse: return "The server returned an invalid response."
        case let .httpStatus(status, message): return "Server error (\(status)): \(message)"
        case .missingToken: return "Sign in before using this feature."
        case .invalidBaseURL: return "The API address is invalid."
        }
    }
}

public protocol PlatformTokenStore: Sendable {
    func read() throws -> String?
    func save(_ token: String) throws
    func clear() throws
}

public final class KeychainTokenStore: PlatformTokenStore, @unchecked Sendable {
    private let service: String
    private let account = "private-platform-session"

    public init(service: String = Bundle.main.bundleIdentifier ?? "private-platform") {
        self.service = service
    }

    public func read() throws -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess, let data = result as? Data,
              let token = String(data: data, encoding: .utf8) else {
            throw PrivatePlatformError.invalidResponse
        }
        return token
    }

    public func save(_ token: String) throws {
        let data = Data(token.utf8)
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        let attributes: [String: Any] = [kSecValueData as String: data]
        let status = SecItemUpdate(query as CFDictionary, attributes as CFDictionary)
        if status == errSecItemNotFound {
            var insert = query
            insert[kSecValueData as String] = data
            let insertStatus = SecItemAdd(insert as CFDictionary, nil)
            guard insertStatus == errSecSuccess else { throw PrivatePlatformError.invalidResponse }
        } else if status != errSecSuccess {
            throw PrivatePlatformError.invalidResponse
        }
    }

    public func clear() throws {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        let status = SecItemDelete(query as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw PrivatePlatformError.invalidResponse
        }
    }
}

public actor PrivatePlatformClient {
    private let baseURL: URL
    private let session: URLSession
    private let tokenStore: any PlatformTokenStore
    private let decoder = JSONDecoder()

    public init(
        baseURL: URL,
        session: URLSession = .shared,
        tokenStore: any PlatformTokenStore = KeychainTokenStore()
    ) {
        self.baseURL = baseURL
        self.session = session
        self.tokenStore = tokenStore
    }

    public func register(username: String, password: String, displayName: String) async throws -> AuthSession {
        let result: AuthSession = try await request(
            "POST", path: "/v1/auth/register",
            body: ["username": username, "password": password, "display_name": displayName],
            authenticated: false
        )
        try tokenStore.save(result.token)
        return result
    }

    public func login(username: String, password: String, deviceName: String = "iOS") async throws -> AuthSession {
        let result: AuthSession = try await request(
            "POST", path: "/v1/auth/login",
            body: ["username": username, "password": password, "device_name": deviceName],
            authenticated: false
        )
        try tokenStore.save(result.token)
        return result
    }

    public func currentUser() async throws -> PlatformUser {
        try await request("GET", path: "/v1/auth/me")
    }

    public func logout() async throws {
        let _: EmptyResponse = try await request("POST", path: "/v1/auth/logout")
        try tokenStore.clear()
    }

    public func searchUsers(query: String) async throws -> [PlatformUser] {
        var allowed = CharacterSet.urlQueryAllowed
        allowed.remove(charactersIn: "&=+#")
        let encoded = query.addingPercentEncoding(withAllowedCharacters: allowed) ?? query
        let result: UserList = try await request("GET", path: "/v1/users/search?q=\(encoded)")
        return result.users
    }

    public func conversations() async throws -> [PlatformConversation] {
        let result: ConversationList = try await request("GET", path: "/v1/conversations")
        return result.conversations
    }

    public func createConversation(kind: String, title: String = "", memberIDs: [String] = []) async throws -> PlatformConversation {
        try await request(
            "POST", path: "/v1/conversations",
            body: ["kind": kind, "title": title, "member_ids": memberIDs]
        )
    }

    public func messages(conversationID: String) async throws -> [PlatformMessage] {
        let result: MessageList = try await request("GET", path: "/v1/conversations/\(conversationID)/messages")
        return result.messages
    }

    public func sendMessage(_ body: String, conversationID: String, replyTo: String? = nil) async throws -> PlatformMessage {
        var payload: [String: Any] = ["body": body]
        if let replyTo { payload["reply_to"] = replyTo }
        return try await request("POST", path: "/v1/conversations/\(conversationID)/messages", body: payload)
    }

    public func wallet() async throws -> WalletSnapshot {
        try await request("GET", path: "/v1/wallet")
    }

    public func gifts() async throws -> [PlatformGift] {
        let result: GiftList = try await request("GET", path: "/v1/gifts", authenticated: false)
        return result.gifts
    }

    public func giftInventory() async throws -> [GiftInventoryItem] {
        let result: GiftInventory = try await request("GET", path: "/v1/gifts/inventory")
        return result.items
    }

    public func purchaseGift(id: String, idempotencyKey: String = UUID().uuidString) async throws -> GiftPurchaseResult {
        try await request("POST", path: "/v1/gifts/\(id)/purchase", idempotencyKey: idempotencyKey)
    }

    public func transferGift(ownershipID: String, to userID: String, idempotencyKey: String = UUID().uuidString) async throws {
        let _: EmptyResponse = try await request(
            "POST", path: "/v1/gifts/ownership/\(ownershipID)/transfer",
            body: ["recipient_user_id": userID], idempotencyKey: idempotencyKey
        )
    }

    public func upgradeGift(ownershipID: String, idempotencyKey: String = UUID().uuidString) async throws {
        let _: UpgradeResponse = try await request(
            "POST", path: "/v1/gifts/ownership/\(ownershipID)/upgrade",
            idempotencyKey: idempotencyKey
        )
    }

    public func adjustStars(
        userID: String,
        amount: Int64,
        reason: String,
        note: String = "",
        idempotencyKey: String = UUID().uuidString
    ) async throws -> AdminWalletAdjustment {
        try await request(
            "POST", path: "/v1/admin/wallet/adjust",
            body: ["user_id": userID, "amount": amount, "reason": reason, "note": note],
            idempotencyKey: idempotencyKey
        )
    }

    public func createGift(
        code: String,
        title: String,
        description: String,
        rarity: String,
        priceStars: Int64,
        upgradeCostStars: Int64,
        maxLevel: Int,
        maxSupply: Int64?
    ) async throws -> PlatformGift {
        var payload: [String: Any] = [
            "code": code, "title": title, "description": description,
            "rarity": rarity, "price_stars": priceStars,
            "upgrade_cost_stars": upgradeCostStars, "max_level": maxLevel
        ]
        payload["max_supply"] = maxSupply
        return try await request("POST", path: "/v1/admin/gifts", body: payload)
    }

    public func issueGift(giftID: String, userID: String, idempotencyKey: String = UUID().uuidString) async throws -> AdminGiftIssueResult {
        try await request(
            "POST", path: "/v1/admin/gifts/issue",
            body: ["gift_id": giftID, "user_id": userID],
            idempotencyKey: idempotencyKey
        )
    }

    private func request<Response: Decodable>(
        _ method: String,
        path: String,
        body: Any? = nil,
        authenticated: Bool = true,
        idempotencyKey: String? = nil
    ) async throws -> Response {
        guard let url = URL(string: path, relativeTo: baseURL)?.absoluteURL else {
            throw PrivatePlatformError.invalidBaseURL
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let body {
            request.httpBody = try JSONSerialization.data(withJSONObject: body)
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        if authenticated {
            guard let token = try tokenStore.read(), !token.isEmpty else {
                throw PrivatePlatformError.missingToken
            }
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        if let idempotencyKey {
            request.setValue(idempotencyKey, forHTTPHeaderField: "Idempotency-Key")
        }
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw PrivatePlatformError.invalidResponse }
        guard (200..<300).contains(http.statusCode) else {
            let payload = (try? JSONSerialization.jsonObject(with: data)) as? [String: String]
            throw PrivatePlatformError.httpStatus(http.statusCode, payload?["error"] ?? "unknown_error")
        }
        if Response.self == EmptyResponse.self, data.isEmpty {
            return EmptyResponse() as! Response
        }
        do { return try decoder.decode(Response.self, from: data) }
        catch { throw PrivatePlatformError.invalidResponse }
    }
}

private struct EmptyResponse: Decodable {}
private struct UserList: Decodable { let users: [PlatformUser] }
private struct ConversationList: Decodable { let conversations: [PlatformConversation] }
private struct MessageList: Decodable { let messages: [PlatformMessage] }
private struct GiftList: Decodable { let gifts: [PlatformGift] }
private struct GiftInventory: Decodable { let items: [GiftInventoryItem] }
public struct AdminGiftIssueResult: Decodable, Sendable {\n    public let ownershipID: String\n    public let giftID: String\n    public let userID: String\n    public let serialNumber: Int64\n    public let level: Int\n    enum CodingKeys: String, CodingKey { case level; case ownershipID = "ownership_id"; case giftID = "gift_id"; case userID = "user_id"; case serialNumber = "serial_number" }\n}\n\npublic struct AdminWalletAdjustment: Decodable, Sendable {\n    public let userID: String\n    public let balance: Int64\n    enum CodingKeys: String, CodingKey { case balance; case userID = "user_id" }\n}\nprivate struct UpgradeResponse: Decodable {
    let ownershipID: String
    let level: Int
    let upgradeCost: Int64
    let balance: Int64
    enum CodingKeys: String, CodingKey {
        case level, balance
        case ownershipID = "ownership_id"
        case upgradeCost = "upgrade_cost"
    }
}
