import Foundation
import UIKit

/// Writing to the system pasteboard, behind a seam so the sync rules can be
/// tested without a real pasteboard. Reading is deliberately absent: iOS asks
/// the user before an app reads what another app copied, so Ferry only ever
/// receives pasteboard content through a system paste control the user taps.
@MainActor
protocol PasteboardWriting {
    func write(text: String)
    func write(imageData: Data) -> Bool
}

@MainActor
struct SystemPasteboard: PasteboardWriting {
    func write(text: String) {
        UIPasteboard.general.string = text
    }

    func write(imageData: Data) -> Bool {
        guard let image = UIImage(data: imageData) else { return false }
        UIPasteboard.general.image = image
        return true
    }
}

/// Fetching the bytes behind a message attachment. This is separate from
/// `FerryServicing` because it is a different capability: the timeline works
/// without it, and only clipboard sync needs to pull an image down.
@MainActor
protocol AttachmentLoading {
    func attachment(endpoint: ServerEndpoint, token: String, path: String) async throws -> Data
}
