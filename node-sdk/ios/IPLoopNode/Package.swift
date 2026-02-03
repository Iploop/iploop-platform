// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "IPLoopNode",
    platforms: [
        .iOS(.v15)
    ],
    products: [
        .library(name: "IPLoopNode", targets: ["IPLoopNode"])
    ],
    dependencies: [
        .package(url: "https://github.com/daltoniam/Starscream.git", from: "4.0.0"),
        .package(url: "https://github.com/Alamofire/Alamofire.git", from: "5.8.0"),
    ],
    targets: [
        .target(
            name: "IPLoopNode",
            dependencies: ["Starscream", "Alamofire"],
            path: "Sources"
        )
    ]
)
