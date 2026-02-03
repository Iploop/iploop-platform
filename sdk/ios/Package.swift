// swift-tools-version:5.7
import PackageDescription

let package = Package(
    name: "IPLoopSDK",
    platforms: [
        .iOS(.v13),
        .macOS(.v10_15)
    ],
    products: [
        .library(
            name: "IPLoopSDK",
            targets: ["IPLoopSDK"]
        ),
    ],
    dependencies: [],
    targets: [
        .target(
            name: "IPLoopSDK",
            dependencies: [],
            path: "Sources/IPLoopSDK"
        ),
        .testTarget(
            name: "IPLoopSDKTests",
            dependencies: ["IPLoopSDK"],
            path: "Tests/IPLoopSDKTests"
        ),
    ]
)
