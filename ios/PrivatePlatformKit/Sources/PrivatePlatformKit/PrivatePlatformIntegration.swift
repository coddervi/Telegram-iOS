import SwiftUI
import UIKit

@MainActor
public func makePrivatePlatformViewController(baseURL: URL) -> UIViewController {
    let client = PrivatePlatformClient(baseURL: baseURL)
    let store = PrivatePlatformStore(client: client)
    let controller = UIHostingController(rootView: PrivatePlatformRootView(store: store))
    controller.modalPresentationStyle = .fullScreen
    return controller
}
