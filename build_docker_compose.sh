#!/bin/bash
# Reproducible build: inject pinned refs and normalize exported timestamps.
set -uo pipefail
cd "$(dirname "$0")" || exit 1

set -a
# shellcheck disable=SC1091
. ./reproducible.env
set +a
export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-0}"
echo "SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}"

[ -d .git ] && find . -path ./.git -prune -o -exec chmod go-w {} +

BAKE_SET=()
while IFS='=' read -r key value; do
  case "$key" in ''|\#*) continue;; esac
  BAKE_SET+=(--set "*.args.${key}=${value}")
done < <(grep -vE '^[[:space:]]*(#|$)' reproducible.env)
BAKE_SET+=(--set "*.args.SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}")

if docker --help | grep -q buildx; then
  BUILDER_NAME="${SPR_HERDR_BUILDER:-super-builder}"
  if [ "$BUILDER_NAME" = "super-builder" ] &&
     docker buildx inspect super-builder >/dev/null 2>&1; then
    CURRENT_BUILDKIT=$(docker buildx inspect super-builder \
      | sed -n 's/.*image="\([^"]*\)".*/\1/p' | head -1)
    if [ -n "${BUILDKIT_REF:-}" ] && [ "$CURRENT_BUILDKIT" != "$BUILDKIT_REF" ]; then
      docker buildx rm super-builder
    fi
  fi
  if [ "$BUILDER_NAME" = "super-builder" ]; then
    docker buildx create --name super-builder --driver docker-container \
      --driver-opt "image=${BUILDKIT_REF}" 2>/dev/null || true
  fi

  OUTPUT="type=docker,rewrite-timestamp=true"
  ARGS=()
  for arg in "$@"; do
    case "$arg" in
      --load) ;;
      --push) OUTPUT="type=registry,rewrite-timestamp=true" ;;
      *) ARGS+=("$arg") ;;
    esac
  done
  docker buildx bake --builder "$BUILDER_NAME" --file docker-compose.yml \
    "${BAKE_SET[@]}" --set "*.output=${OUTPUT}" ${ARGS[@]+"${ARGS[@]}"}
else
  export DOCKER_BUILDKIT=1
  export COMPOSE_DOCKER_CLI_BUILD=1
  docker compose build "$@"
fi

status=$?
if [ "$status" -ne 0 ]; then
  echo "Tip: if the build failed to resolve domain names, run" >&2
  echo "./base/docker_nftables_setup.sh from the SPR checkout." >&2
fi
exit "$status"
