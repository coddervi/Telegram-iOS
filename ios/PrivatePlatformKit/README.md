# PrivatePlatformKit

Native Swift client and SwiftUI starter screens for the independent private platform API.

## Requirements

- iOS 16+
- Swift 5.9+
- A deployed instance of the Go API from `/backend`

## Add to an Xcode project

Add this repository package in Xcode using the local package path `ios/PrivatePlatformKit`, or add the package URL after the repository is available at its final branch/tag.

For local development on the same Wi-Fi network as the Windows server (currently `192.168.1.74`), use the development endpoint:

```swift
import PrivatePlatformKit
import SwiftUI

let api = PrivatePlatformClient(
    baseURL: PrivatePlatformConfiguration.localDevelopmentBaseURL
)
let store = PrivatePlatformStore(client: api)

PrivatePlatformRootView(store: store)
```

You can check connectivity before presenting the root view with `try await api.health()`. The endpoint must be reachable from the iPhone. The local address is specific to the current LAN and may change if the PC receives a different DHCP address.

The sample root view includes account registration/login, chat list, direct-chat user search, text messaging with photo attachments, internal Stars balance, gift catalog, inventory, gift upgrades and a basic admin panel for issuing gifts and adjusting Stars. Accounts with the server-side `admin` role also see an admin screen for Stars adjustments and gift publishing.

Session tokens are persisted in the iOS Keychain. Do not ship a development HTTP endpoint or database credentials in the app. Use HTTPS in production.

## API coverage

The client currently wraps authentication, user search, conversations, text messages, wallet history, gift catalog, purchase, inventory, transfer and upgrade endpoints.

This package is a native integration layer and starter UI. In the `private-platform-foundation` branch it is wired into the TelegramUI Bazel target and presented from the app's scene attach path as the launch UI, replacing the Telegram entry screen. The private platform API is separate from Telegram's MTProto service. The integration has not been build-validated yet.

The local HTTP endpoint is for LAN development only. Do not ship it in a public release; use HTTPS in production.
