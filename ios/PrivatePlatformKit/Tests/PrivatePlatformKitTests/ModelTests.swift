import XCTest
@testable import PrivatePlatformKit

final class ModelTests: XCTestCase {
    func testSearchUserCanOmitRole() throws {
        let data = Data(#"{"id":"u1","username":"alice","display_name":"Alice","bio":"hello"}"#.utf8)
        let user = try JSONDecoder().decode(PlatformUser.self, from: data)
        XCTAssertEqual(user.username, "alice")
        XCTAssertNil(user.role)
    }

    func testMessageDecodesMediaAttachments() throws {
        let data = Data(#"{"id":"m1","conversation_id":"c1","sender_id":"u1","body":"","reply_to":null,"media_ids":["media1"],"created_at":"2026-01-01T00:00:00Z"}"#.utf8)
        let message = try JSONDecoder().decode(PlatformMessage.self, from: data)
        XCTAssertEqual(message.mediaIDs, ["media1"])
    }

    func testUploadedMediaUsesBackendKeys() throws {
        let data = Data(#"{"id":"media1","content_type":"image/png","size_bytes":123}"#.utf8)
        let media = try JSONDecoder().decode(UploadedMedia.self, from: data)
        XCTAssertEqual(media.contentType, "image/png")
        XCTAssertEqual(media.sizeBytes, 123)
    }

    func testGiftUsesBackendSnakeCaseKeys() throws {
        let data = Data(#"{"id":"g1","code":"rose","title":"Rose","description":"","rarity":"rare","price_stars":50,"upgrade_cost_stars":100,"max_level":5,"max_supply":100,"issued_count":2}"#.utf8)
        let gift = try JSONDecoder().decode(PlatformGift.self, from: data)
        XCTAssertEqual(gift.priceStars, 50)
        XCTAssertEqual(gift.maxLevel, 5)
    }
}
