#!/usr/bin/env bash
# Refresh container pins and the latest stable Herdr Linux release metadata.
set -euo pipefail
cd "$(dirname "$0")"

DOCKERFILE_TAG=docker/dockerfile:1
BUILDKIT_TAG=moby/buildkit:buildx-stable-1
NODE_TAG=node:18
GO_TAG=golang:1.26.5-alpine
SPR_KRUN_PLUGIN_TAG=ghcr.io/spr-networks/spr-krun-plugin:latest
UBUNTU_SNAPSHOT="${UBUNTU_SNAPSHOT:-$(sed -n 's/^UBUNTU_SNAPSHOT=//p' reproducible.env)}"
NERD_FONTS_VERSION=$(sed -n 's/^NERD_FONTS_VERSION=//p' reproducible.env)
NERD_FONTS_JETBRAINS_MONO_ARCHIVE_SHA256=$(sed -n 's/^NERD_FONTS_JETBRAINS_MONO_ARCHIVE_SHA256=//p' reproducible.env)
JETBRAINS_MONO_NERD_REGULAR_WOFF2_SHA256=$(sed -n 's/^JETBRAINS_MONO_NERD_REGULAR_WOFF2_SHA256=//p' reproducible.env)
JETBRAINS_MONO_NERD_BOLD_WOFF2_SHA256=$(sed -n 's/^JETBRAINS_MONO_NERD_BOLD_WOFF2_SHA256=//p' reproducible.env)

mdigest() {
  docker buildx imagetools inspect "$1" --format '{{.Manifest.Digest}}'
}

echo "Resolving image digests..." >&2
DOCKERFILE_SYNTAX="${DOCKERFILE_TAG}@$(mdigest "$DOCKERFILE_TAG")"
BUILDKIT_REF="${BUILDKIT_TAG}@$(mdigest "$BUILDKIT_TAG")"
NODE_REF="${NODE_TAG}@$(mdigest "$NODE_TAG")"
GO_REF="${GO_TAG}@$(mdigest "$GO_TAG")"
SPR_KRUN_PLUGIN_REF="${SPR_KRUN_PLUGIN_TAG%:*}@$(mdigest "$SPR_KRUN_PLUGIN_TAG")"

release=$(curl -fsSL https://api.github.com/repos/herdrdev/herdr/releases/latest)
HERDR_VERSION=$(jq -r '.tag_name | sub("^v"; "")' <<<"$release")
HERDR_SHA256_AMD64=$(jq -r '.assets[] | select(.name == "herdr-linux-x86_64") | .digest | sub("^sha256:"; "")' <<<"$release")
HERDR_SHA256_ARM64=$(jq -r '.assets[] | select(.name == "herdr-linux-aarch64") | .digest | sub("^sha256:"; "")' <<<"$release")
for value in "$HERDR_SHA256_AMD64" "$HERDR_SHA256_ARM64"; do
  [[ "$value" =~ ^[0-9a-f]{64}$ ]] || { echo "Herdr release is missing a Linux asset digest" >&2; exit 1; }
done

starship_release=$(curl -fsSL https://api.github.com/repos/starship/starship/releases/latest)
STARSHIP_VERSION=$(jq -r '.tag_name | sub("^v"; "")' <<<"$starship_release")
STARSHIP_SHA256_AMD64=$(jq -r '.assets[] | select(.name == "starship-x86_64-unknown-linux-musl.tar.gz") | .digest | sub("^sha256:"; "")' <<<"$starship_release")
STARSHIP_SHA256_ARM64=$(jq -r '.assets[] | select(.name == "starship-aarch64-unknown-linux-musl.tar.gz") | .digest | sub("^sha256:"; "")' <<<"$starship_release")
for value in "$STARSHIP_SHA256_AMD64" "$STARSHIP_SHA256_ARM64"; do
  [[ "$value" =~ ^[0-9a-f]{64}$ ]] || { echo "Starship release is missing a static Linux asset digest" >&2; exit 1; }
done

tag_ref=$(curl -fsSL "https://api.github.com/repos/herdrdev/herdr/git/ref/tags/v${HERDR_VERSION}")
tag_sha=$(jq -r '.object.sha' <<<"$tag_ref")
tag_type=$(jq -r '.object.type' <<<"$tag_ref")
if [ "$tag_type" = "tag" ]; then
  tag_object=$(curl -fsSL "https://api.github.com/repos/herdrdev/herdr/git/tags/${tag_sha}")
  HERDR_COMMIT=$(jq -r '.object.sha' <<<"$tag_object")
else
  HERDR_COMMIT="$tag_sha"
fi
[[ "$HERDR_COMMIT" =~ ^[0-9a-f]{40}$ ]] || { echo "Could not resolve the Herdr source commit" >&2; exit 1; }

status=$(curl -fsS -o /dev/null -w '%{http_code}' \
  "https://snapshot.ubuntu.com/ubuntu/${UBUNTU_SNAPSHOT}/dists/noble/InRelease" || true)
[ "$status" = "200" ] || { echo "Ubuntu snapshot is unavailable (HTTP ${status})" >&2; exit 1; }

scratch=$(mktemp)
trap 'rm -f "$scratch"' EXIT
cat >"$scratch" <<EOF
# Pinned build inputs for build_docker_compose.sh and CI. Regenerate with ./update-pins.sh.
DOCKERFILE_SYNTAX=${DOCKERFILE_SYNTAX}
BUILDKIT_REF=${BUILDKIT_REF}
NODE_REF=${NODE_REF}
GO_REF=${GO_REF}
SPR_KRUN_PLUGIN_REF=${SPR_KRUN_PLUGIN_REF}
UBUNTU_SNAPSHOT=${UBUNTU_SNAPSHOT}
# Official stable Herdr release from https://github.com/herdrdev/herdr.
HERDR_VERSION=${HERDR_VERSION}
HERDR_COMMIT=${HERDR_COMMIT}
HERDR_SHA256_AMD64=${HERDR_SHA256_AMD64}
HERDR_SHA256_ARM64=${HERDR_SHA256_ARM64}
# Official static Starship release from https://github.com/starship/starship.
STARSHIP_VERSION=${STARSHIP_VERSION}
STARSHIP_SHA256_AMD64=${STARSHIP_SHA256_AMD64}
STARSHIP_SHA256_ARM64=${STARSHIP_SHA256_ARM64}
# Vendored from the official Nerd Fonts release archive.
NERD_FONTS_VERSION=${NERD_FONTS_VERSION}
NERD_FONTS_JETBRAINS_MONO_ARCHIVE_SHA256=${NERD_FONTS_JETBRAINS_MONO_ARCHIVE_SHA256}
JETBRAINS_MONO_NERD_REGULAR_WOFF2_SHA256=${JETBRAINS_MONO_NERD_REGULAR_WOFF2_SHA256}
JETBRAINS_MONO_NERD_BOLD_WOFF2_SHA256=${JETBRAINS_MONO_NERD_BOLD_WOFF2_SHA256}
EOF
mv "$scratch" reproducible.env
chmod 0644 reproducible.env
trap - EXIT

replace_line() {
  local file="$1" pattern="$2" replacement="$3" temp
  temp=$(mktemp)
  sed "s|${pattern}|${replacement}|" "$file" >"$temp"
  chmod 0644 "$temp"
  mv "$temp" "$file"
}

replace_line Dockerfile '^# syntax=.*' "# syntax=${DOCKERFILE_SYNTAX}"
for key in NODE_REF GO_REF SPR_KRUN_PLUGIN_REF HERDR_VERSION HERDR_COMMIT HERDR_SHA256_AMD64 HERDR_SHA256_ARM64 STARSHIP_VERSION STARSHIP_SHA256_AMD64 STARSHIP_SHA256_ARM64 UBUNTU_SNAPSHOT; do
  value=${!key}
  replace_line Dockerfile "^ARG ${key}=.*" "ARG ${key}=${value}"
done

echo "Pinned Herdr ${HERDR_VERSION} and Starship ${STARSHIP_VERSION}. Review with git diff." >&2
