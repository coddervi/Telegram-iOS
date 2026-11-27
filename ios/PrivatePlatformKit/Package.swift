// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "PrivatePlatformKit",
    platforms: [.iOS(.v16)],
    products: [
        .library(name: "PrivatePlatformKit", targets: ["PrivatePlatformKit"])
    ],
    targets: [
        .target(name: "PrivatePlatformKit"),
        .testTarget(name: "PrivatePlatformKitTests", dependencies: ["PrivatePlatformKit"])
    ]
)
