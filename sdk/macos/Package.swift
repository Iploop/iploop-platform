// swift-tools-version:5.7
import PackageDescription

let package = Package(
    name: "IPLoopSDK",
    platforms: [
        .macOS(.v11)
    ],
    products: [
        .library(
            name: "IPLoopSDK",
            targets: ["IPLoopSDK"]
        ),
        .executable(
            name: "iploop-daemon",
            targets: ["IPLoopDaemon"]
        )
    ],
    dependencies: [],
    targets: [
        .target(
            name: "IPLoopSDK",
            dependencies: [],
            path: "Sources/IPLoopSDK"
        ),
        .executableTarget(
            name: "IPLoopDaemon",
            dependencies: ["IPLoopSDK"],
            path: "Sources/IPLoopDaemon"
        )
    ]
)
