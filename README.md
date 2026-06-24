# GoShell

Accessible SSH server manager. Native desktop window (no browser tab, no console window),
built with a local HTTP/WebSocket backend rendered through [glaze](https://github.com/crgimenes/glaze)
(WebView2 on Windows, WebKitGTK on Linux, WebKit on macOS — all CGO-free).

Because the UI is just ARIA-annotated HTML inside a native WebView, screen readers
(NVDA on Windows, Orca on Linux) read it the same way they read any accessible web page —
no GTK/AT-SPI bridging headaches, no custom UIA wiring.

## Why this architecture (not GTK)

GTK (gotk3) only gets you AT-SPI accessibility on Linux. On Windows, GTK has no real NVDA
story — you'd need MSYS2-bundled DLLs and NVDA still wouldn't read most of it properly.
glaze uses the OS's native WebView (WebView2/Edge on Windows), which *does* speak UI Automation,
so NVDA works out of the box, on both platforms, from one HTML/JS frontend.

## Architecture

Most of the app is plain request/response (connect, list services, read a file,
chmod, save settings, …) — for that, `internal/ui/service.go` exposes a single
`Service` struct whose every exported method becomes `window.goshell_<method>()`
in JS automatically via `glaze.BindMethods`. No routes, no JSON encode/decode
boilerplate, no fetch wrapper: the JS bridge handles all of that, and the
binding's Promise rejects with the Go error directly.

Two things are genuinely *not* request/response — live apt output/progress, and
the interactive terminal, where stdin keeps flowing while stdout keeps streaming
in. `Bind` has no server-push primitive (confirmed against glaze's own examples:
even its REPL example just blocks until the whole result is ready), so those two
go over real WebSockets in `internal/ui/server.go`, served by a small `net/http`
server we start ourselves on a random loopback port. `glaze.New()` then opens a
window pointed at that local URL and binds `Service` to it.

(Note: `glaze.AppWindow`, the higher-level "just hand me an http.Handler"
helper, was considered and dropped — it creates the window internally and
never exposes the `WebView` value, so `Bind` isn't reachable through it.)

## Build

Requires Go 1.26+ (glaze's minimum). On Linux you'll also need WebKitGTK dev headers
at runtime (most distros ship this already); on Windows, the Edge WebView2 Runtime
(preinstalled on Windows 10/11).

`go.sum` is intentionally not committed — this sandbox's network blocks
`proxy.golang.org`/`golang.org`, so any `go.sum` generated here would be
checksummed against substitute mirrors, not the real modules. Run `go mod tidy`
once on a normal machine and commit the result.

```bash
git clone https://github.com/denizsincar29/goshell.git
cd goshell
go mod tidy
go build -o goshell ./cmd/goshell
./goshell
```

On Windows, build with `-ldflags="-H windowsgui"` to hide the console window:

```bash
go build -ldflags="-H windowsgui" -o goshell.exe ./cmd/goshell
```

## Features

- **Connect**: pick a host from `~/.ssh/config`, a previously saved host, or enter details
  manually. Keys are auto-discovered from `~/.ssh/`.
- **Services**: list/start/stop/restart/enable/disable systemd services, view journal logs,
  with or without sudo.
- **Crontab**: load/edit/save crontabs for any user (sudo for others).
- **Files**: browse the remote filesystem, edit files directly over SSH (no scp/sftp —
  uses base64-safe `cat`/`mv` so encoding never breaks), chmod/chown with optional recursion.
- **Resources**: live CPU/RAM/swap/disk usage and top processes, with auto-refresh.
- **Updates**: `apt-get update`/`upgrade`/`dist-upgrade` with a real progress bar, live
  streamed output, and a config-conflict policy you choose up front (no surprise dpkg prompts).
- **Terminal**: run arbitrary commands, sudo optional.

All remote operations are plain SSH commands — nothing is installed on the server.

## Accessibility notes

- Every control has a real `<label>`, semantic role, and visible focus outline.
- Status changes are announced via `aria-live` regions rather than relying on visual cues alone.
- Tab list follows the standard ARIA tabs keyboard pattern (arrow keys to move, click/Enter to activate).
- Color is never the only signal (active/failed services are also labeled in text).
- Respects `prefers-reduced-motion` and `prefers-contrast`.

## Config

Settings and saved hosts persist to `~/.config/goshell/config.json`.
