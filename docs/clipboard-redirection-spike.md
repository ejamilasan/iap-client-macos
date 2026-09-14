# Clipboard redirection spike

## Outcome

The embedded RDP viewer cannot gain real clipboard sharing through an app-only
change. The pinned `github.com/nakagami/grdp v0.3.9` client does not register the
RDP `cliprdr` virtual channel, and its unfinished clipboard implementation is
coupled to Windows `user32` and OLE APIs.

The recommended MVP is bidirectional plain-text clipboard redirection. It
requires a small fork of `grdp` that exposes a platform-neutral clipboard
adapter, followed by Wails integration in this repository. Image and file
clipboard formats should be deferred.

## Current behavior

There are two RDP paths:

1. External Microsoft Remote Desktop/Windows App sessions generate an `.rdp`
   file containing `redirectclipboard:i:1`. Clipboard sharing is already
   enabled there when `RDPSettings.ClipboardShare` is true.
2. Embedded sessions create `grdp.RdpClient` directly. They only send pointer
   and keyboard input and receive bitmap frames. `RDPSettings.ClipboardShare`
   is not passed into this path.

The React canvas prevents the browser's default key handling and forwards key
scan codes. Consequently, Command-V does not read the macOS clipboard, and the
embedded client has no channel through which either endpoint can announce or
request clipboard data.

## Why enabling the existing code is insufficient

In `grdp v0.3.9`:

- `RdpClient.Login` has both `cliprdr` registration calls commented out.
- `MCSClient.SetClientCliprdr` does not add the virtual channel because its only
  implementation line is commented out.
- `plugin/cliprdr/cliprdr.go` mixes RDP protocol messages with local clipboard
  access.
- `plugin/cliprdr/cliprdr_windows.go` implements that access using Windows-only
  packages and APIs. There is no Darwin implementation.

Simply uncommenting registration would therefore fail to build on macOS and
would not provide an application callback API.

## Proposed MVP architecture

Support only `CF_UNICODETEXT` initially.

```text
macOS pasteboard
      ^  |
      |  v
Wails ClipboardGetText / ClipboardSetText
      ^  |
      |  v
RDPViewerService clipboard adapter
      ^  |
      |  v
forked grdp cliprdr client
      ^  |
      |  v
Windows RDP clipboard virtual channel
```

### Required `grdp` fork changes

1. Add a platform-neutral interface, for example:

   ```go
   type Clipboard interface {
       ReadText() (string, error)
       WriteText(string) error
       OnTextChanged(func()) (stop func(), err error)
   }
   ```

2. Refactor `plugin/cliprdr` so protocol encoding/decoding does not call
   Windows APIs. The text-only transport should:

   - advertise `CF_UNICODETEXT` after `CB_MONITOR_READY`;
   - answer `CB_FORMAT_DATA_REQUEST` with UTF-16LE text and a terminating NUL;
   - process a server `CB_FORMAT_LIST`, request `CF_UNICODETEXT`, decode the
     response, and call `WriteText`;
   - announce a new format list when `OnTextChanged` reports a local change;
   - suppress echo loops when a remote value is written locally;
   - reject malformed lengths and cap clipboard payload size.

3. Restore virtual-channel negotiation:

   ```go
   g.channels.Register(cliprdr.NewClient(clipboard))
   g.mcs.SetClientCliprdr()
   ```

   `SetClientCliprdr` must call
   `clientNetworkData.AddVirtualChannel(cliprdr.ChannelName,
   cliprdr.ChannelOption)`.

4. Expose clipboard configuration before `Login`, for example
   `NewRdpClient(..., grdp.WithClipboard(adapter))`. A disabled or nil adapter
   must not advertise `cliprdr`.

5. Add protocol-level tests using a fake clipboard adapter. Do not require a
   GUI clipboard or an RDP server for unit tests.

### Required application changes

1. Pass `clipboardShare` into `StartViewerSession` and
   `ConnectSessionWithResolution`. Today the embedded path never receives it.
2. Implement the adapter using Wails clipboard calls on the UI side, bridged by
   session-specific request/result events. Clipboard access may need to execute
   on the Wails/UI thread rather than from the RDP network goroutine.
3. Keep clipboard state per RDP session, but coordinate writes through the one
   host pasteboard. Only the active session should publish local clipboard
   changes to prevent inactive sessions from receiving sensitive data.
4. Honor `RDPSettings.ClipboardShare`; default remains `true`, matching external
   sessions.
5. Show a toolbar indicator or toggle so clipboard sharing is visible during a
   session.

## Event/API sketch

Application bindings:

```go
func (a *App) StartViewerSession(
    sessionID, tunnelID, username, password string,
    width, height int,
    clipboardShare bool,
) error

func (a *App) ViewerSessionClipboardChanged(sessionID, text string) error
func (a *App) ViewerSessionSetActive(sessionID string) error
```

Events from Go to React:

- `rdp-clipboard-read-<sessionID>`: ask React/Wails for current local text.
- `rdp-clipboard-write-<sessionID>`: set text received from Windows.
- `rdp-clipboard-error-<sessionID>`: non-fatal clipboard failure.

The generated Wails runtime already provides `ClipboardGetText` and
`ClipboardSetText`, so no native macOS package should be needed in this app.

## Security and behavior constraints

- Clipboard sharing must be explicitly controlled by the saved connection
  setting and visibly indicated in-session.
- Limit the MVP to text and cap values (for example, 1 MiB) to avoid excessive
  memory use.
- Clear cached clipboard contents and stop watchers on disconnect.
- Do not log clipboard contents.
- Treat RDP server policy that disables clipboard redirection as a normal,
  non-fatal condition.
- Avoid polling while the app is in the background where possible. If Wails
  offers no pasteboard-change notification, poll only for the active session at
  a modest interval and compare a hash/value before announcing changes.

## Verification plan

1. Unit-test `cliprdr` capability, format-list, request, response, UTF-16, size
   limit, malformed input, and echo-suppression behavior.
2. Build and test the fork on Darwin and Windows to prevent platform-specific
   regressions.
3. Connect to a Windows VM with clipboard redirection allowed and verify:

   - macOS plain text copies into Notepad via Ctrl-V;
   - Windows plain text copies into a macOS editor via Command-V;
   - Unicode, multiline text, and an empty clipboard;
   - disabled `clipboardShare` negotiates no channel;
   - two open sessions only synchronize the active session;
   - reconnect/disconnect leaves no watcher or goroutine running.

4. Verify external Microsoft Remote Desktop launch remains unchanged.

## Estimated scope and decision

This is a dependency-plus-application feature, not a one-line RDP setting.
Expect roughly 3-5 engineering days for a robust text-only MVP plus live Windows
interoperability testing. Image and file clipboard support would be a separate,
substantially larger follow-up.

Recommended decision: fork `grdp` for a text-only, adapter-based `cliprdr`
implementation. If maintaining an RDP protocol fork is undesirable, retain the
external Windows App path for clipboard-enabled sessions or replace `grdp` with
a maintained client/library that already supports `cliprdr` on macOS.
