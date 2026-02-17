# IAP Client for macOS

A native macOS desktop application for connecting to Google Cloud Windows VMs through Identity-Aware Proxy (IAP) tunnels. Built with Go and React using the Wails framework.

## Features

- **IAP Tunneling** -- Create secure IAP tunnels to GCP VM instances using the native [cedws/iapc](https://github.com/cedws/iapc) library
- **GCP Project Browser** -- Browse your accessible GCP projects and list VM instances across all zones
- **Windows VM Filtering** -- Filter instance lists to show only Windows VMs
- **Multi-Session Support** -- Open multiple concurrent RDP sessions in tabs and switch between them
- **Embedded RDP Viewer** -- View remote desktops directly in the app using an embedded RDP client (via [nakagami/grdp](https://github.com/nicholasgasior/grdp))
- **Microsoft Remote Desktop Integration** -- Fallback option to launch sessions in Microsoft Remote Desktop
- **Connection Profiles** -- Save, edit, and manage connection profiles for frequently accessed VMs
- **Secure Credential Storage** -- Store passwords in the macOS Keychain via [go-keyring](https://github.com/zalando/go-keyring)
- **Windows Password Reset** -- Reset Windows user passwords on GCP instances via the serial port protocol
- **Auto-Close Idle Tunnels** -- Tunnels automatically close after 2 minutes of inactivity to prevent resource leaks
- **Dark Mode** -- Toggle between light and dark themes
- **gcloud CLI Authentication** -- Authenticate using `gcloud auth application-default login` from your terminal

## Prerequisites

- macOS 12+ (Monterey or later)
- [Google Cloud SDK (gcloud CLI)](https://cloud.google.com/sdk/docs/install)
- Google Cloud account with IAP-secured tunnel access
- [Microsoft Remote Desktop](https://apps.apple.com/app/microsoft-remote-desktop/id1295203466) (optional, for external RDP sessions)

## Installation

### From DMG (recommended)

1. Download the latest `.dmg` from the [releases page](https://github.com/your-org/iap-desktop-macos/releases)
2. Open the DMG and drag **IAP Client** to your Applications folder
3. Right-click the app and select **Open** (required for unsigned apps on first launch)
4. Click **Open** in the security dialog

### From Source

Requirements:

- [Go 1.23+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/) and npm
- Xcode Command Line Tools (`xcode-select --install`)

```bash
# Clone and set up
git clone https://github.com/your-org/iap-desktop-macos.git
cd iap-desktop-macos
make install-deps

# Ensure ~/go/bin is in your PATH (add to ~/.zshrc to make permanent)
export PATH="$HOME/go/bin:$PATH"
```

Then choose **one** of the following depending on what you need:

```bash
# Option A: Run in development mode (hot reload, for making changes)
make dev

# Option B: Build the app (output: build/bin/IAP Client.app)
make build

# Option C: Build a DMG installer (output: build/dmg/IAP Client.dmg)
make build-dmg
```

`make build-dmg` runs `make build` automatically, so you don't need to run both. Once built, open `build/bin/IAP Client.app` directly, or install from the DMG in `build/dmg/`.

To build a universal binary (Intel + Apple Silicon):

```bash
make build-universal      # .app only
make build-dmg-universal  # .app + .dmg
```

## Usage

### 1. Authenticate

Open a terminal and run:

```bash
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/compute
```

This stores Application Default Credentials that the app uses to access GCP APIs.

### 2. Launch the App

Open **IAP Client** from your Applications folder (or run `make dev` during development). If you authenticated before launching, the app will detect your credentials automatically. Otherwise, click **Refresh Auth** after authenticating.

### 3. Browse and Connect

1. Select a GCP project from the sidebar dropdown
2. Browse available VM instances (toggle **Windows only** to filter)
3. Click the connect button on a running VM
4. Enter your Windows username and password in the connection form
5. The app creates an IAP tunnel and opens an RDP session in a new tab

### 4. Manage Connections

- Save connection profiles for quick access to frequently used VMs
- Stored passwords are kept in the macOS Keychain
- Edit or delete saved connections from the sidebar

## Development

### Project Structure

```
iap-desktop-macos/
├── main.go                    # Wails entry point
├── app.go                     # App struct with Wails method bindings
├── internal/
│   ├── auth/                  # gcloud / ADC authentication
│   ├── gcp/                   # GCP project and instance APIs
│   ├── tunnel/                # IAP tunnel management
│   ├── keychain/              # macOS Keychain wrapper
│   ├── rdp/                   # RDP file generation and password reset
│   ├── config/                # Connection and settings persistence
│   └── viewer/                # Embedded RDP viewer (grdp)
├── frontend/
│   └── src/
│       ├── App.tsx            # Main React app
│       ├── App.css            # Styles
│       ├── types/             # TypeScript interfaces
│       └── components/
│           ├── AuthStatus.tsx       # Auth status and refresh
│           ├── Sidebar.tsx          # Project and connection browser
│           ├── ProjectBrowser.tsx   # Project selection
│           ├── ConnectionForm.tsx   # Connection setup form
│           ├── ConnectionList.tsx   # Saved connections
│           ├── SessionTabs.tsx      # Multi-session tab bar
│           ├── SessionViewer.tsx    # Active session display
│           ├── RDPViewer.tsx        # Embedded RDP canvas
│           └── TunnelStatus.tsx     # Tunnel info display
├── build/darwin/
│   ├── entitlements.plist     # macOS sandbox entitlements
│   └── Info.plist             # App bundle metadata
├── wails.json                 # Wails framework config
├── go.mod / go.sum            # Go dependencies
├── Makefile                   # Build and dev commands
└── README.md
```

### Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.23 |
| Frontend | React 18 + TypeScript |
| Bundler | Vite |
| Desktop Framework | Wails v2 |
| IAP Tunneling | cedws/iapc |
| RDP Client | nakagami/grdp |
| GCP APIs | google.golang.org/api |
| Auth | golang.org/x/oauth2 + gcloud ADC |
| Keychain | zalando/go-keyring |

### Make Commands

```
make install-deps        Install all dependencies (Go, npm, Wails CLI)
make dev                 Run in development mode with hot reload
make build               Build for production (current architecture)
make build-universal     Build universal binary (Intel + Apple Silicon)
make build-dmg           Build and create DMG installer
make build-dmg-universal Build universal DMG installer
make clean               Clean all build artifacts
make test                Run Go tests
make lint                Run linters (golangci-lint + eslint)
make fmt                 Format code (go fmt + prettier)
make security-check      Run gosec security scanner
make update-deps         Update Go and npm dependencies
make help                Show available commands
```

## License

MIT

## Credits

- [Wails](https://wails.io/) -- Desktop app framework for Go
- [cedws/iapc](https://github.com/cedws/iapc) -- Native IAP tunneling library
- [nakagami/grdp](https://github.com/nicholasgasior/grdp) -- Go RDP protocol implementation
- [zalando/go-keyring](https://github.com/zalando/go-keyring) -- macOS Keychain integration
