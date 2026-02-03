import XCTest
@testable import IPLoopSDK

final class IPLoopSDKTests: XCTestCase {
    
    override func setUp() {
        super.setUp()
        // Reset state before each test
        UserDefaults.standard.removeObject(forKey: "iploop_user_consent")
    }
    
    func testInitialization() {
        let sdk = IPLoopSDK.shared
        sdk.initialize(apiKey: "test_api_key_12345")
        
        // SDK should be initialized but not running
        XCTAssertFalse(sdk.isActive)
    }
    
    func testUserConsent() {
        let sdk = IPLoopSDK.shared
        
        // Initially no consent
        XCTAssertFalse(sdk.hasUserConsent())
        
        // Set consent
        sdk.setUserConsent(true)
        XCTAssertTrue(sdk.hasUserConsent())
        
        // Revoke consent
        sdk.setUserConsent(false)
        XCTAssertFalse(sdk.hasUserConsent())
    }
    
    func testBandwidthStats() {
        let sdk = IPLoopSDK.shared
        let stats = sdk.getBandwidthStats()
        
        // Initial stats should be zero
        XCTAssertEqual(stats.totalBytes, 0)
        XCTAssertEqual(stats.totalRequests, 0)
        XCTAssertEqual(stats.totalMB, 0.0)
    }
    
    func testStartWithoutInitialization() async {
        let sdk = IPLoopSDK.shared
        
        // Should throw error when starting without initialization
        do {
            try await sdk.start()
            XCTFail("Should have thrown an error")
        } catch {
            // Expected
            XCTAssertTrue(error is IPLoopError)
        }
    }
}
