# spr-herdr

Run [Herdr](https://herdr.dev/) inside an SPR-managed KVM microVM and use its
terminal UI directly from the sandboxed SPR plugin pane.

NOTE: This is an autocoded LLM generated plugin for Herdr. 


## Sandboxed terminal UI

Herdr is a TUI, so this plugin embeds xterm.js in SPR's
`sandbox="allow-scripts"` iframe. A small Go service owns one persistent PTY,
launches the Herdr client, and serves the terminal UI over a guest-local Unix
socket:

```text
SPR sandboxed iframe
  -> short-lived, spr-herdr-scoped SPR UI credential
  -> /plugins/spr-herdr/terminal/*
  -> /state/plugins/spr-herdr/socket.sock on the SPR host
  -> libkrun vsock port 4040
  -> /run/spr-herdr/ui.sock in the guest supervisor
  -> spr-herdr-terminal -> PTY -> Bubblewrap -> herdr
```

Output uses an authenticated long poll with an absolute replay cursor. Input
and resize events use ordered POST requests. This is deliberate: SPR's current
generic plugin reverse proxy clears WebSocket upgrade headers and its dedicated
plugin WebSocket route is not enabled. The HTTP transport provides keyboard,
UTF-8, resize, ANSI/true-color, alternate-screen, and mouse-event behavior
without changing SPR core or opening a listener on the LAN.

The terminal service keeps a 4 MiB output ring. Reloading the SPR page
reconstructs the current screen from that ring and reuses the same PTY. If two
browser tabs open the plugin, they mirror the same terminal; input from both is
serialized into that shared PTY.

Herdr and every shell or agent it starts run in a second layer of Linux
namespaces created by Bubblewrap. That process tree receives an empty `/run`,
a private `/proc`, a minimal `/dev`, a read-only view of the image, and writable
access only to its persistent home and private temporary files. The terminal
supervisor remains outside that namespace so it can own the guest-local UI
socket; the Herdr process tree cannot see that socket or the runtime-injected
`/run/spr-krun` mount. SPR's socket path and vsock-port environment variables
are also removed before Herdr starts.

The terminal bundles JetBrainsMono Nerd Font and uses Omarchy's default Tokyo
Night palette. Herdr follows that ANSI palette with Omarchy's compact pane
layout: blue accents, shared split borders, and no pane scrollbars. The result
does not depend on fonts installed in the viewing browser.

## Install and use

In SPR, open **Plugins → + New Plugin** and enter the repository URL for
`spr-herdr`. SPR selects `docker-compose-kvm.yml`, creates the private plugin
network, applies the manifest's stable virtual-device identity, and exposes the
guest UI only through the plugin Unix socket.

Open **spr-herdr** in the sidebar. Herdr's first-run setup appears inside the
terminal. Its standard prefix is `Ctrl+B`; press and release it, then press the
action key. For example:

- `Ctrl+B`, then `c` creates a tab.
- `Ctrl+B`, then `v` splits side by side.
- `Ctrl+B`, then `-` splits horizontally.
- `Ctrl+B`, then `q` detaches the client. The bridge automatically reattaches.

The **Reattach** button restarts only the browser-facing Herdr client. It does
not ask the Herdr server to stop, so live panes remain running.

The guest includes Bash, Starship, Git, OpenSSH, curl, jq, ripgrep, editors, and
common shell utilities. New homes receive Omarchy's minimal Starship prompt;
existing `.bashrc` and `~/.config/starship.toml` files are never overwritten.
Agent CLIs are intentionally not baked into the image. Install additional
user-local tools beneath `/home/herdr`, or use Herdr's native remote mode to
attach to a development host:

```sh
herdr --remote workbox
```

SSH configuration, keys, Herdr configuration/session data, plugins, cloned
repositories, and shell history persist under SPR's plugin configuration tree:

```text
configs/plugins/spr-herdr/
```

Treat that directory like terminal history: pane contents and credentials may
be sensitive. Back it up and protect it accordingly.

The host-visible Unix socket remains under `state/plugins/spr-herdr/`; it is
runtime transport state and does not contain the persistent Herdr home.

## Isolation and runtime

- `Runtime: kvm` with libkrun; defaults to 2 vCPUs and 1024 MiB RAM. Override
  with `SPR_KRUN_CPUS` and `SPR_KRUN_RAM_MIB`.
- The TUI, terminal transport, and API are confined to the SPR Unix socket →
  vsock path. Compose has no `ports`, `expose`, or host networking.
- `SandboxedUI` is `true`. The iframe has scripts but no same-origin access to
  the SPR application. SPR injects a short-lived token scoped only to
  `/plugins/spr-herdr`.
- The terminal service and Herdr run as UID/GID 10000 with all Linux
  capabilities removed and `no-new-privileges` enabled.
- Herdr and all descendants additionally run inside Bubblewrap mount, user,
  PID, IPC, and UTS namespaces. `/run` and `/proc` are replaced rather than
  inherited, while networking remains shared so the existing SPR device policy
  continues to apply. Failure to create the sandbox prevents Herdr from
  starting.
- The guest appears as one SPR-managed device with MAC
  `02:53:50:52:4b:16`, private interface `spr-herdr`, and only the declared
  `wan` and `dns` policies.
- Herdr's background binary version check is disabled because the executable is
  image-pinned. Remote agent-manifest updates stay enabled. Upgrade Herdr by
  updating the plugin image pins.

Text paste works through the browser terminal. Herdr's host-native image
clipboard bridge is not available from the sandboxed iframe.

## Build and test

Build and load the multi-stage image with:

```sh
./build_docker_compose.sh --load
```

Run manifest, Compose, Go, and frontend checks with:

```sh
./test.sh
```

`reproducible.env` pins Dockerfile/BuildKit, Node, Go, the SPR krun base (and
therefore SPR's `container_template`), the Ubuntu package snapshot, Herdr
version/source commit, both official Linux asset hashes, and the vendored Nerd
Font source/output hashes. The static Starship version and architecture hashes
are pinned as well. Refresh current stable container, Herdr, and Starship pins
with:

```sh
./update-pins.sh
git diff
```

The Docker build installs frontend dependencies from `package-lock.json`,
builds the xterm.js assets, runs the Go tests, compiles a static terminal
bridge, downloads only the target-architecture Herdr release binary, verifies
its digest, and copies those artifacts into the final `spr-krun-plugin`
runtime. That runtime inherits SPR's `container_template`, including its CA
bundle and guest conventions.

## Upstream and licenses

- [herdrdev/herdr](https://github.com/herdrdev/herdr), Apache-2.0 at the pinned
  `0.8.0` release. The binary is unmodified and this plugin is not affiliated
  with the Herdr project.
- [xtermjs/xterm.js](https://github.com/xtermjs/xterm.js), MIT.
- [ryanoasis/nerd-fonts](https://github.com/ryanoasis/nerd-fonts), JetBrainsMono
  Nerd Font 3.5.0 under the SIL Open Font License 1.1.
- [starship/starship](https://github.com/starship/starship), ISC.
- [creack/pty](https://github.com/creack/pty), MIT.
- [containers/bubblewrap](https://github.com/containers/bubblewrap),
  LGPL-2.0-or-later, installed from the pinned Ubuntu snapshot.
- [spr-networks/spr-krun-plugin](https://github.com/spr-networks/spr-krun-plugin),
  the guest-side vsock bridge and init contract.

The SPR integration code is MIT-licensed. Third-party notices and license texts
are included in `NOTICE` and `LICENSES/`.
