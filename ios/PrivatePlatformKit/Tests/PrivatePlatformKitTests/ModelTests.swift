import XCTest
@testable import PrivatePlatformKit

final class ModelTests: XCTestCase {
    func testSearchUserCanOmitRole() throws {
        let data = Data(#"{"id":"u1","username":"alice","display_name":"Alice","bio":"hello"}"#.utf8)
        let user = try JSONDecoder().decode(PlatformUser.self, from: data)
        XCTAssertEqual(user.username, "alice")
        XCTAssertNil(user.role)
    }

    func testGiftUsesBackendSnakeCaseKeys() throws {
        let data = Data(#"{"id":"g1","code":"rose","title":"Rose","description":"","rarity":"rare","price_stars":50,"upgrade_cost_stars":100,"max_level":5,"max_supply":100,"issued_count":2}"#.utf8)
        let gift = try JSONDecoder().decode(PlatformGift.self, from: data)
        XCTAssertEqual(gift.priceStars, 50)
        XCTAssertEqual(gift.maxLevel, 5)
    }
}
