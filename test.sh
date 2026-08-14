#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"

jq -e '
  .Name == "spr-herdr" and
  .Runtime == "kvm" and
  .UnixPath == "/state/plugins/spr-herdr/socket.sock" and
  .URI == "spr-herdr" and
  .HasUI == true and
  .SandboxedUI == true and
  .NetworkCapabilities.Interface == "spr-herdr" and
  .NetworkCapabilities.DeviceMAC == "02:53:50:52:4b:16" and
  (.NetworkCapabilities.Policies | sort) == ["dns", "wan"]
' plugin.json >/dev/null

grep -Fq 'krun.vsock_path: "/state/plugins/spr-herdr/socket.sock"' docker-compose-kvm.yml
grep -Fq 'SPR_KRUN_PLUGIN_SOCKET: /run/spr-herdr/ui.sock' docker-compose-kvm.yml
grep -Fq 'com.docker.network.bridge.inhibit_ipv4: "true"' docker-compose-kvm.yml
grep -Fq '${SUPERDIR:-/home/spr/super}/configs/plugins/spr-herdr:/home/herdr' docker-compose.yml
if grep -Fq '/state/plugins/spr-herdr/home:/home/herdr' docker-compose.yml; then
  echo "spr-herdr home must live in configs/plugins, not state/plugins" >&2
  exit 1
fi
grep -Fq 'FROM ${SPR_KRUN_PLUGIN_REF} AS krun-plugin' Dockerfile
grep -Fq 'FROM krun-plugin' Dockerfile
if grep -Eq '^ARG (UBUNTU_REF|ALPINE_REF)=|^FROM \$\{(UBUNTU_REF|ALPINE_REF)\}' Dockerfile; then
  echo "spr-herdr must inherit its runtime and CA bundle from spr-krun-plugin" >&2
  exit 1
fi
if grep -Eq '(^|[[:space:]])ports:|(^|[[:space:]])network_mode:[[:space:]]*host' docker-compose*.yml; then
  echo "spr-herdr must not publish ports or use host networking" >&2
  exit 1
fi

bash -n build_docker_compose.sh config/bashrc scripts/spr-herdr-init update-pins.sh

test_cache="${TMPDIR:-/tmp}/spr-herdr-go-cache"
module_cache="${TMPDIR:-/tmp}/spr-herdr-go-mod-cache"
mkdir -p "$test_cache" "$module_cache"
(
  cd code
  env GOCACHE="$test_cache" GOMODCACHE="$module_cache" go test ./...
)

if [ ! -d frontend/node_modules ]; then
  npm --prefix frontend ci --ignore-scripts --no-audit --no-fund
fi
npm --prefix frontend run build
test -s frontend/dist/index.html
test -s frontend/dist/app.js
test -s frontend/dist/app.css
test "$(find frontend/dist/assets -type f -name 'JetBrainsMonoNerdFont-Regular-*.woff2' | wc -l | tr -d ' ')" = "1"
test "$(find frontend/dist/assets -type f -name 'JetBrainsMonoNerdFont-Bold-*.woff2' | wc -l | tr -d ' ')" = "1"

grep -Fq 'name = "terminal"' config/config.toml
grep -Fq 'accent = "blue"' config/config.toml
grep -Fq 'pane_gaps = false' config/config.toml
grep -Fq 'pane_scrollbars = false' config/config.toml
grep -Fq "background: '#1a1b26'" frontend/src/app.js
grep -Fq 'format = "[$directory$git_branch$git_status]($style)$character"' config/starship.toml
grep -Fq 'if [ ! -e /home/herdr/.bashrc ]; then' scripts/spr-herdr-init
grep -Fq 'if [ ! -e /home/herdr/.config/starship.toml ]; then' scripts/spr-herdr-init

echo 'e65d5843cf1281526c5c4d6d44f0c3a1672efc8c8903d4c7ee973396b5e388db  frontend/src/fonts/JetBrainsMonoNerdFont-Regular.woff2' | shasum -a 256 -c - >/dev/null
echo '19f08a54ba2f3584fb3640f47ee28d2d90d75d581cc714006417dd65e32a1415  frontend/src/fonts/JetBrainsMonoNerdFont-Bold.woff2' | shasum -a 256 -c - >/dev/null

grep -Fq 'HERDR_VERSION=0.8.0' reproducible.env
grep -Fq 'HERDR_COMMIT=346411fa21afd297f5ed3b3fa56f9e3fbf7654b7' reproducible.env
grep -Fq 'b872ea7e40fa2cb17e857ac9b62b1bf26db7b403c622f5d2f3f5b35f6e9acd28' reproducible.env
grep -Fq 'f647ac66468d9efbc642fe534fb284468f0aea60641606fc008dfc0d82a3ca87' reproducible.env
grep -Fq 'NERD_FONTS_VERSION=3.5.0' reproducible.env
grep -Fq 'STARSHIP_VERSION=1.26.0' reproducible.env
grep -Fq 'b7c232b0e8249d8e55a40beb79c5c43a7d370f3f9408bd215deb0170daeaadf3' reproducible.env
grep -Fq 'dc30189378d2f2e287384e8a692d3f95ad1df64cf0e8c36aa9201516028aed6b' reproducible.env

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  SUPERDIR=/tmp/spr-super docker compose -f docker-compose.yml config >/dev/null
  SUPERDIR=/tmp/spr-super docker compose -f docker-compose-kvm.yml config >/dev/null
fi

echo "spr-herdr checks passed"
