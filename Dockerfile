# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89
ARG NODE_REF=node:18@sha256:c6ae79e38498325db67193d391e6ec1d224d96c693a8a4d943498556716d3783
ARG GO_REF=golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2
ARG SPR_KRUN_PLUGIN_REF=ghcr.io/spr-networks/spr-krun-plugin@sha256:5fa6ba286065ab016ffd2c3ce9c9bd627973150197fbd3db56e77d33f25f04a7
ARG HERDR_VERSION=0.8.0
ARG HERDR_COMMIT=346411fa21afd297f5ed3b3fa56f9e3fbf7654b7
ARG HERDR_SHA256_AMD64=b872ea7e40fa2cb17e857ac9b62b1bf26db7b403c622f5d2f3f5b35f6e9acd28
ARG HERDR_SHA256_ARM64=f647ac66468d9efbc642fe534fb284468f0aea60641606fc008dfc0d82a3ca87
ARG STARSHIP_VERSION=1.26.0
ARG STARSHIP_SHA256_AMD64=b7c232b0e8249d8e55a40beb79c5c43a7d370f3f9408bd215deb0170daeaadf3
ARG STARSHIP_SHA256_ARM64=dc30189378d2f2e287384e8a692d3f95ad1df64cf0e8c36aa9201516028aed6b
ARG UBUNTU_SNAPSHOT=20260601T000000Z

# spr-krun-plugin inherits SPR's container_template. Use it as the shared
# runtime base so the guest CA bundle, init contract, and base utilities stay
# aligned with other SPR KVM plugins.
FROM ${SPR_KRUN_PLUGIN_REF} AS krun-plugin

FROM ${NODE_REF} AS frontend
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json frontend/build.mjs ./
COPY frontend/src/ ./src/
RUN --mount=type=tmpfs,target=/root/.npm \
    --mount=type=tmpfs,target=/frontend/node_modules \
    npm ci --ignore-scripts --no-audit --no-fund && \
    npm run build && \
    test -s dist/index.html && test -s dist/app.js && test -s dist/app.css

FROM ${GO_REF} AS terminal-builder
WORKDIR /src
COPY code/go.mod code/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY code/ ./
COPY --from=frontend /frontend/dist/ ./ui/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=tmpfs,target=/root/.cache/go-build \
    go test ./... && \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /spr-herdr-terminal .

FROM krun-plugin AS herdr-downloader
ARG TARGETARCH
ARG HERDR_VERSION
ARG HERDR_SHA256_AMD64
ARG HERDR_SHA256_ARM64
ARG STARSHIP_VERSION
ARG STARSHIP_SHA256_AMD64
ARG STARSHIP_SHA256_ARM64
ARG UBUNTU_SNAPSHOT
RUN set -eux; \
    printf 'Types: deb\nURIs: https://snapshot.ubuntu.com/ubuntu/%s\nSuites: noble noble-updates noble-security\nComponents: main restricted universe multiverse\nSigned-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n' "${UBUNTU_SNAPSHOT}" > /etc/apt/sources.list.d/ubuntu.sources; \
    printf 'APT::Install-Recommends "false";\nAcquire::Check-Valid-Until "false";\n' > /etc/apt/apt.conf.d/99reproducible; \
    apt-get update; \
    apt-get install -y --no-install-recommends wget; \
    rm -rf /var/lib/apt/lists/* /var/log/* /var/cache/ldconfig/aux-cache
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) HERDR_ASSET=herdr-linux-x86_64; HERDR_SHA256="${HERDR_SHA256_AMD64}" ;; \
      arm64) HERDR_ASSET=herdr-linux-aarch64; HERDR_SHA256="${HERDR_SHA256_ARM64}" ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -q "https://github.com/herdrdev/herdr/releases/download/v${HERDR_VERSION}/${HERDR_ASSET}" -O /herdr; \
    echo "${HERDR_SHA256}  /herdr" | sha256sum -c -; \
    chmod 0755 /herdr
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) STARSHIP_ARCH=x86_64; STARSHIP_SHA256="${STARSHIP_SHA256_AMD64}" ;; \
      arm64) STARSHIP_ARCH=aarch64; STARSHIP_SHA256="${STARSHIP_SHA256_ARM64}" ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -q "https://github.com/starship/starship/releases/download/v${STARSHIP_VERSION}/starship-${STARSHIP_ARCH}-unknown-linux-musl.tar.gz" -O /starship.tar.gz; \
    echo "${STARSHIP_SHA256}  /starship.tar.gz" | sha256sum -c -; \
    tar -xzf /starship.tar.gz -C /tmp starship; \
    install -m 0755 /tmp/starship /starship

FROM krun-plugin
ARG HERDR_VERSION
ARG HERDR_COMMIT
ARG STARSHIP_VERSION
ARG UBUNTU_SNAPSHOT
ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/herdr \
    SHELL=/bin/bash \
    TERM=xterm-256color \
    COLORTERM=truecolor \
    LANG=C.UTF-8 \
    HERDR_CONFIG_PATH=/home/herdr/.config/herdr/config.toml \
    HERDR_DISABLE_SOUND=1 \
    HERDR_VERSION=${HERDR_VERSION}
RUN set -eux; \
    printf 'Types: deb\nURIs: https://snapshot.ubuntu.com/ubuntu/%s\nSuites: noble noble-updates noble-security\nComponents: main restricted universe multiverse\nSigned-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n' "${UBUNTU_SNAPSHOT}" > /etc/apt/sources.list.d/ubuntu.sources; \
    printf 'APT::Install-Recommends "false";\nAcquire::Check-Valid-Until "false";\n' > /etc/apt/apt.conf.d/99reproducible; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      curl git jq less nano openssh-client passwd procps \
      ripgrep tzdata unzip vim-tiny; \
    rm -rf /var/lib/apt/lists/* /var/log/* /var/cache/ldconfig/aux-cache; \
    groupadd --gid 10000 herdr; \
    useradd --uid 10000 --gid herdr --create-home --home-dir /home/herdr --shell /bin/bash herdr; \
    install -d -m 0750 -o herdr -g herdr /home/herdr/workspace /home/herdr/.config/herdr

COPY --from=herdr-downloader /herdr /usr/local/bin/herdr
COPY --from=herdr-downloader /starship /usr/local/bin/starship
COPY --from=terminal-builder /spr-herdr-terminal /usr/local/bin/spr-herdr-terminal
COPY --chmod=0755 scripts/spr-herdr-init /usr/local/bin/spr-herdr-init
COPY config/ /usr/share/spr-herdr/
COPY LICENSE NOTICE /usr/share/doc/spr-herdr/
COPY LICENSES/ /usr/share/doc/spr-herdr/LICENSES/

LABEL org.opencontainers.image.source="https://github.com/spr-networks/spr-herdr" \
      org.opencontainers.image.description="Herdr TUI in an SPR-managed KVM microVM" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${HERDR_VERSION}" \
      org.opencontainers.image.revision="${HERDR_COMMIT}" \
      org.opencontainers.image.vendor.starship-version="${STARSHIP_VERSION}"

VOLUME ["/home/herdr"]
WORKDIR /home/herdr/workspace
USER root
HEALTHCHECK NONE
ENTRYPOINT ["/usr/local/bin/spr-herdr-init"]
