import XCTest

@MainActor
final class FerryUITests: XCTestCase {
    func testMacminiDeploymentJourney() throws {
        let bundle = Bundle(for: Self.self)
        let server = try XCTUnwrap(bundle.object(forInfoDictionaryKey: "FERRY_UI_SERVER") as? String)
        let password = bundle.object(forInfoDictionaryKey: "FERRY_UI_PASSWORD") as? String ?? ""
        let app = XCUIApplication()
        app.launch()

        replace(app.textFields["server-address"], with: server)
        replace(app.textFields["device-name"], with: "iPhone 17 Pro")
        replace(app.secureTextFields["access-password"], with: password)
        app.buttons["connect-device"].tap()

        let currentDevice = app.staticTexts["current-device"]
        XCTAssertTrue(currentDevice.waitForExistence(timeout: 10))
        XCTAssertEqual(currentDevice.label, "iPhone 17 Pro")

        let input = app.textFields["message-input"]
        XCTAssertTrue(input.waitForExistence(timeout: 3))
        input.tap()
        input.typeText("from iphone via macmini 42817")
        app.buttons["send-message"].tap()
        XCTAssertTrue(message("from iphone via macmini 42817", in: app).waitForExistence(timeout: 10))
        XCTAssertTrue(message("from web via macmini 42817", in: app).waitForExistence(timeout: 60))
    }

    func testRealServerJourney() throws {
        let bundle = Bundle(for: Self.self)
        let server = try XCTUnwrap(bundle.object(forInfoDictionaryKey: "FERRY_UI_SERVER") as? String)
        let password = bundle.object(forInfoDictionaryKey: "FERRY_UI_PASSWORD") as? String ?? ""
        let app = XCUIApplication()
        app.launch()

        replace(app.textFields["server-address"], with: server)
        replace(app.textFields["device-name"], with: "iPhone UI Test")
        replace(app.secureTextFields["access-password"], with: password)
        app.buttons["connect-device"].tap()

        let currentDevice = app.staticTexts["current-device"]
        XCTAssertTrue(currentDevice.waitForExistence(timeout: 10))
        XCTAssertEqual(currentDevice.label, "iPhone UI Test")
        let input = app.textFields["message-input"]
        XCTAssertTrue(input.waitForExistence(timeout: 3))
        input.tap()
        input.typeText("hello from iOS")
        app.buttons["send-message"].tap()
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "identifier == 'message-text' AND label == 'hello from iOS'")).firstMatch.waitForExistence(timeout: 10))

        app.buttons["attach-file"].tap()
        selectFixture(in: app)
        XCTAssertTrue(app.descendants(matching: .any)["selected-file"].waitForExistence(timeout: 5))
        app.buttons["send-message"].tap()
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "identifier == 'message-file-name' AND label == 'ios-fixture.txt'")).firstMatch.waitForExistence(timeout: 10))

        XCTAssertTrue(app.secureTextFields["access-password"].waitForExistence(timeout: 30))
        XCTAssertEqual(app.descendants(matching: .any)["access-error"].label, "This device is no longer connected.")
    }

    private func replace(_ field: XCUIElement, with value: String) {
        XCTAssertTrue(field.waitForExistence(timeout: 5))
        field.tap()
        let existingCount = (field.value as? String)?.count ?? 0
        field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: existingCount) + value)
    }

    private func message(_ body: String, in app: XCUIApplication) -> XCUIElement {
        app.staticTexts.matching(NSPredicate(format: "identifier == 'message-text' AND label == %@", body)).firstMatch
    }

    private func selectFixture(in app: XCUIApplication) {
        let fixture = app.cells.matching(NSPredicate(format: "identifier BEGINSWITH 'ios-fixture'")).firstMatch
        if !fixture.waitForExistence(timeout: 3) {
            let browse = app.buttons["Browse"].firstMatch
            if browse.exists { browse.tap() }
            let onMyPhone = app.staticTexts["On My iPhone"].firstMatch
            if onMyPhone.waitForExistence(timeout: 3) { onMyPhone.tap() }
            let ferry = app.staticTexts["Ferry"].firstMatch
            if ferry.waitForExistence(timeout: 3) { ferry.tap() }
        }
        XCTAssertTrue(fixture.waitForExistence(timeout: 20), "The seeded fixture must appear in the system document picker")
        fixture.tap()
    }
}
