import SwiftUI

@MainActor
public final class PrivatePlatformStore: ObservableObject {
    @Published public private(set) var user: PlatformUser?
    @Published public private(set) var conversations: [PlatformConversation] = []
    @Published public private(set) var messages: [String: [PlatformMessage]] = [:]
    @Published public private(set) var wallet: WalletSnapshot?
    @Published public private(set) var gifts: [PlatformGift] = []
    @Published public private(set) var inventory: [GiftInventoryItem] = []
    @Published public private(set) var searchResults: [PlatformUser] = []
    @Published public var errorMessage: String?

    public let client: PrivatePlatformClient

    public init(client: PrivatePlatformClient) {
        self.client = client
    }

    public func bootstrap() async {
        do {
            user = try await client.currentUser()
            await refresh()
        } catch {
            user = nil
        }
    }

    public func login(username: String, password: String) async {
        do {
            let session = try await client.login(username: username, password: password)
            user = session.user
            await refresh()
        } catch { errorMessage = error.localizedDescription }
    }

    public func register(username: String, password: String, displayName: String) async {
        do {
            let session = try await client.register(username: username, password: password, displayName: displayName)
            user = session.user
            await refresh()
        } catch { errorMessage = error.localizedDescription }
    }

    public func logout() async {
        do { try await client.logout() } catch { errorMessage = error.localizedDescription }
        user = nil
        conversations = []
        messages = [:]
        wallet = nil
        inventory = []
    }

    public func refresh() async {
        do {
            async let chats = client.conversations()
            async let balance = client.wallet()
            async let catalog = client.gifts()
            async let owned = client.giftInventory()
            conversations = try await chats
            wallet = try await balance
            gifts = try await catalog
            inventory = try await owned
        } catch { errorMessage = error.localizedDescription }
    }

    public func searchUsers(_ query: String) async {
        guard query.count >= 2 else { searchResults = []; return }
        do { searchResults = try await client.searchUsers(query: query) }
        catch { errorMessage = error.localizedDescription }
    }

    public func startDirectChat(with user: PlatformUser) async {
        do {
            _ = try await client.createConversation(kind: "direct", memberIDs: [user.id])
            conversations = try await client.conversations()
        } catch { errorMessage = error.localizedDescription }
    }

    public func loadMessages(for conversation: PlatformConversation) async {
        do { messages[conversation.id] = try await client.messages(conversationID: conversation.id) }
        catch { errorMessage = error.localizedDescription }
    }

    public func send(_ text: String, to conversation: PlatformConversation) async {
        do {
            _ = try await client.sendMessage(text, conversationID: conversation.id)
            messages[conversation.id] = try await client.messages(conversationID: conversation.id)
        } catch { errorMessage = error.localizedDescription }
    }

    public func purchase(_ gift: PlatformGift) async {
        do {
            _ = try await client.purchaseGift(id: gift.id)
            await refresh()
        } catch { errorMessage = error.localizedDescription }
    }

    public func upgrade(_ item: GiftInventoryItem) async {
        do {
            try await client.upgradeGift(ownershipID: item.ownershipID)
            await refresh()
        } catch { errorMessage = error.localizedDescription }
    }

    public func adminAdjustStars(userID: String, amount: Int64, reason: String, note: String) async {
        do {
            _ = try await client.adjustStars(userID: userID, amount: amount, reason: reason, note: note)
            await refresh()
        } catch { errorMessage = error.localizedDescription }
    }

    public func adminCreateGift(code: String, title: String, rarity: String, price: Int64, upgradeCost: Int64, maxLevel: Int, maxSupply: Int64?) async {
        do {
            _ = try await client.createGift(code: code, title: title, description: "", rarity: rarity, priceStars: price, upgradeCostStars: upgradeCost, maxLevel: maxLevel, maxSupply: maxSupply)
            await refresh()
        } catch { errorMessage = error.localizedDescription }
    }

    public func adminIssueGift(giftID: String, userID: String) async {
        do {
            _ = try await client.issueGift(giftID: giftID, userID: userID)
            await refresh()
        } catch { errorMessage = error.localizedDescription }
    }
}

public struct PrivatePlatformRootView: View {
    @StateObject private var store: PrivatePlatformStore

    public init(store: PrivatePlatformStore) {
        _store = StateObject(wrappedValue: store)
    }

    public var body: some View {
        Group {
            if store.user == nil {
                PrivatePlatformAuthView(store: store)
            } else {
                TabView {
                    PrivatePlatformChatsView(store: store)
                        .tabItem { Label("Chats", systemImage: "bubble.left.and.bubble.right") }
                    PrivatePlatformGiftsView(store: store)
                        .tabItem { Label("Gifts", systemImage: "gift") }
                    PrivatePlatformWalletView(store: store)
                        .tabItem { Label("Stars", systemImage: "star") }
                    PrivatePlatformProfileView(store: store)
                        .tabItem { Label("Profile", systemImage: "person.crop.circle") }
                }
            }
        }
        .task { await store.bootstrap() }
        .alert("Private Platform", isPresented: Binding(
            get: { store.errorMessage != nil },
            set: { if !$0 { store.errorMessage = nil } }
        )) {
            Button("OK", role: .cancel) { store.errorMessage = nil }
        } message: {
            Text(store.errorMessage ?? "")
        }
    }
}

private struct PrivatePlatformAuthView: View {
    @ObservedObject var store: PrivatePlatformStore
    @State private var username = ""
    @State private var password = ""
    @State private var displayName = ""
    @State private var isRegistering = false
    @State private var isWorking = false

    var body: some View {
        NavigationStack {
            Form {
                Section("Your account") {
                    TextField("Username", text: $username)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    if isRegistering { TextField("Display name", text: $displayName) }
                    SecureField("Password (10+ characters)", text: $password)
                }
                Section {
                    Button(isRegistering ? "Create account" : "Sign in") {
                        Task {
                            isWorking = true
                            if isRegistering {
                                await store.register(username: username, password: password, displayName: displayName)
                            } else {
                                await store.login(username: username, password: password)
                            }
                            isWorking = false
                        }
                    }
                    .disabled(isWorking || username.isEmpty || password.isEmpty)
                    Button(isRegistering ? "I already have an account" : "Create a new account") {
                        isRegistering.toggle()
                    }
                    .buttonStyle(.borderless)
                }
            }
            .navigationTitle("Private Platform")
        }
    }
}

private struct PrivatePlatformChatsView: View {
    @ObservedObject var store: PrivatePlatformStore
    @State private var showingSearch = false

    var body: some View {
        NavigationStack {
            List(store.conversations) { chat in
                NavigationLink {
                    PrivatePlatformConversationView(store: store, conversation: chat)
                } label: {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(chat.title ?? (chat.kind == "direct" ? "Direct chat" : chat.kind.capitalized))
                            .font(.headline)
                        Text(chat.kind.capitalized)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            .overlay {
                if store.conversations.isEmpty {
                    VStack(spacing: 8) { Image(systemName: "bubble.left.and.bubble.right").font(.largeTitle); Text("No conversations").font(.headline); Text("Start a private conversation.").foregroundStyle(.secondary) }
                }
            }
            .navigationTitle("Chats")
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button { showingSearch = true } label: { Image(systemName: "square.and.pencil") }
                }
                ToolbarItem(placement: .navigationBarLeading) {
                    Button { Task { await store.refresh() } } label: { Image(systemName: "arrow.clockwise") }
                }
            }
            .sheet(isPresented: $showingSearch) {
                PrivatePlatformUserSearchView(store: store)
            }
        }
    }
}

private struct PrivatePlatformUserSearchView: View {
    @ObservedObject var store: PrivatePlatformStore
    @Environment(\.dismiss) private var dismiss
    @State private var query = ""

    var body: some View {
        NavigationStack {
            List(store.searchResults) { user in
                Button {
                    Task {
                        await store.startDirectChat(with: user)
                        dismiss()
                    }
                } label: {
                    VStack(alignment: .leading) {
                        Text(user.displayName).foregroundStyle(.primary)
                        Text("@\(user.username)").font(.caption).foregroundStyle(.secondary)
                    }
                }
            }
            .searchable(text: $query, prompt: "Search by username")
            .onChange(of: query) { value in Task { await store.searchUsers(value) } }
            .navigationTitle("New chat")
            .toolbar { ToolbarItem(placement: .topBarTrailing) { Button("Done") { dismiss() } } }
        }
    }
}

private struct PrivatePlatformConversationView: View {
    @ObservedObject var store: PrivatePlatformStore
    let conversation: PlatformConversation
    @State private var draft = ""

    var body: some View {
        VStack(spacing: 0) {
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 10) {
                    ForEach(store.messages[conversation.id] ?? []) { message in
                        VStack(alignment: .leading, spacing: 4) {
                            Text(message.senderID == store.user?.id ? "You" : message.senderID)
                                .font(.caption).foregroundStyle(.secondary)
                            Text(message.body)
                                .padding(10)
                                .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 14))
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
                .padding()
            }
            HStack {
                TextField("Message", text: $draft, axis: .vertical)
                    .textFieldStyle(.roundedBorder)
                Button {
                    let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
                    guard !text.isEmpty else { return }
                    draft = ""
                    Task { await store.send(text, to: conversation) }
                } label: { Image(systemName: "arrow.up.circle.fill").font(.title2) }
                .disabled(draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
            .padding()
        }
        .navigationTitle(conversation.title ?? "Conversation")
        .navigationBarTitleDisplayMode(.inline)
        .task { await store.loadMessages(for: conversation) }
    }
}

private struct PrivatePlatformGiftsView: View {
    @ObservedObject var store: PrivatePlatformStore

    var body: some View {
        NavigationStack {
            List {
                Section("Gift shop") {
                    ForEach(store.gifts) { gift in
                        VStack(alignment: .leading, spacing: 6) {
                            HStack {
                                Text(gift.title).font(.headline)
                                Spacer()
                                Text(gift.rarity.capitalized).font(.caption).foregroundStyle(.secondary)
                            }
                            Text(gift.description).font(.subheadline).foregroundStyle(.secondary)
                            HStack {
                                Text("\(gift.priceStars) ★")
                                Spacer()
                                if let cap = gift.maxSupply {
                                    Text("\(gift.issuedCount)/\(cap) issued").font(.caption).foregroundStyle(.secondary)
                                }
                                Button("Get") { Task { await store.purchase(gift) } }
                                    .buttonStyle(.borderedProminent)
                            }
                        }
                        .padding(.vertical, 4)
                    }
                }
                Section("Your collection") {
                    ForEach(store.inventory) { item in
                        HStack {
                            VStack(alignment: .leading) {
                                Text(item.title).font(.headline)
                                Text("#\(item.serialNumber) · Level \(item.level) · \(item.rarity)")
                                    .font(.caption).foregroundStyle(.secondary)
                            }
                            Spacer()
                            Button("Upgrade") { Task { await store.upgrade(item) } }
                                .buttonStyle(.bordered)
                        }
                    }
                }
            }
            .navigationTitle("Gifts")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button { Task { await store.refresh() } } label: { Image(systemName: "arrow.clockwise") }
                }
            }
        }
    }
}

private struct PrivatePlatformWalletView: View {
    @ObservedObject var store: PrivatePlatformStore

    var body: some View {
        NavigationStack {
            List {
                Section {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("Internal Stars").font(.headline).foregroundStyle(.secondary)
                        Text("\(store.wallet?.balance ?? 0) ★")
                            .font(.system(size: 38, weight: .bold, design: .rounded))
                    }
                    .padding(.vertical, 8)
                }
                Section("Recent activity") {
                    ForEach(store.wallet?.transactions ?? []) { item in
                        HStack {
                            VStack(alignment: .leading) {
                                Text(item.reason.replacingOccurrences(of: "_", with: " ").capitalized)
                                Text(item.createdAt).font(.caption).foregroundStyle(.secondary)
                            }
                            Spacer()
                            Text("\(item.amount > 0 ? "+" : "")\(item.amount) ★")
                                .monospacedDigit()
                        }
                    }
                }
            }
            .navigationTitle("Stars")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button { Task { await store.refresh() } } label: { Image(systemName: "arrow.clockwise") }
                }
            }
        }
    }
}

private struct PrivatePlatformProfileView: View {
    @ObservedObject var store: PrivatePlatformStore

    var body: some View {
        NavigationStack {
            Form {
                Section("Account") {
                    LabeledContent("Name", value: store.user?.displayName ?? "")
                    LabeledContent("Username", value: "@\(store.user?.username ?? "")")
                    LabeledContent("Role", value: store.user?.role ?? "user")
                }
                if store.user?.role == "admin" {
                    Section("Administration") {
                        NavigationLink("Admin panel") {
                            PrivatePlatformAdminView(store: store)
                        }
                    }
                }
                Section {
                    Button("Sign out", role: .destructive) { Task { await store.logout() } }
                }
            }
            .navigationTitle("Profile")
        }
    }
}


private struct PrivatePlatformAdminView: View {
    @ObservedObject var store: PrivatePlatformStore
    @State private var targetUserID = ""
    @State private var amount = "100"
    @State private var reason = "admin_credit"
    @State private var note = ""
    @State private var giftCode = ""
    @State private var giftTitle = ""
    @State private var rarity = "common"
    @State private var price = "100"
    @State private var upgradeCost = "100"
    @State private var maxLevel = "5"
    @State private var maxSupply = ""
    @State private var issueGiftID = ""
    @State private var issueUserID = ""

    var body: some View {
        Form {
            Section("Credit / debit internal Stars") {
                TextField("User ID", text: $targetUserID)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                TextField("Amount (+/-)", text: $amount)
                    .keyboardType(.numbersAndPunctuation)
                TextField("Reason", text: $reason)
                TextField("Audit note", text: $note)
                Button("Apply balance change") {
                    guard let value = Int64(amount), !targetUserID.isEmpty else { return }
                    Task { await store.adminAdjustStars(userID: targetUserID, amount: value, reason: reason, note: note) }
                }
                .disabled(Int64(amount) == nil || targetUserID.isEmpty || Int64(amount) == 0)
            }
            Section("Issue gift to user") {
                TextField("Gift ID", text: $issueGiftID)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                TextField("Recipient user ID", text: $issueUserID)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                Button("Issue gift") {
                    Task { await store.adminIssueGift(giftID: issueGiftID, userID: issueUserID) }
                }
                .disabled(issueGiftID.isEmpty || issueUserID.isEmpty)
            }
            Section("Create gift") {
                TextField("Gift code", text: $giftCode)
                    .textInputAutocapitalization(.never)
                TextField("Gift title", text: $giftTitle)
                Picker("Rarity", selection: $rarity) {
                    ForEach(["common", "uncommon", "rare", "epic", "legendary"], id: \.self) { Text($0.capitalized).tag($0) }
                }
                TextField("Price in Stars", text: $price).keyboardType(.numberPad)
                TextField("Upgrade cost", text: $upgradeCost).keyboardType(.numberPad)
                TextField("Maximum level", text: $maxLevel).keyboardType(.numberPad)
                TextField("Supply cap (empty = unlimited)", text: $maxSupply).keyboardType(.numberPad)
                Button("Publish gift") {
                    guard let priceValue = Int64(price),
                          let upgradeValue = Int64(upgradeCost),
                          let levelValue = Int(maxLevel),
                          maxSupply.isEmpty || Int64(maxSupply) != nil,
                          !giftCode.isEmpty, !giftTitle.isEmpty else { return }
                    Task {
                        await store.adminCreateGift(
                            code: giftCode, title: giftTitle, rarity: rarity,
                            price: priceValue, upgradeCost: upgradeValue,
                            maxLevel: levelValue, maxSupply: Int64(maxSupply)
                        )
                    }
                }
                .disabled(giftCode.isEmpty || giftTitle.isEmpty || Int64(price) == nil || Int64(upgradeCost) == nil || Int(maxLevel) == nil)
            }
        }
        .navigationTitle("Admin panel")
    }
}
