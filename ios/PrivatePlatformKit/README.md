# PrivatePlatformKit

Native Swift client and SwiftUI starter screens for the independent private platform API.

## Requirements

- iOS 16+
- Swift 5.9+
- A deployed instance of the Go API from `/backend`

## Add to an Xcode project

Add this repository package in Xcode using the local package path `ios/PrivatePlatformKit`, or add the package URL after the repository is available at its final branch/tag.

Create the client with the HTTPS URL of your own API:

```swift
import PrivatePlatformKit
import SwiftUI

let api = PrivatePlatformClient(
    baseURL: URL(string: "https://api.example.com")!
)
let store = PrivatePlatformStore(client: api)

PrivatePlatformRootView(store: store)
```

The sample root view includes account registration/login, chat list, direct-chat user search, text messaging, internal Stars balance, gift catalog, inventory and gift upgrades. Accounts with the server-side `admin` role also see an admin screen for Stars adjustments and gift publishing.

Session tokens are persisted in the iOS Keychain. Do not ship a development HTTP endpoint or database credentials in the app. Use HTTPS in production.

## API coverage

The client currently wraps authentication, user search, conversations, text messages, wallet history, gift catalog, purchase, inventory, transfer and upgrade endpoints.

This package is a native integration layer and starter UI. It has not yet been wired into the upstream Telegram application target; that requires changing the app's account/session and messaging navigation flows to use this platform instead of Telegram's MTProto stack.
