ARG API_BUILD_IMAGE=golang:1.26-alpine
FROM ${API_BUILD_IMAGE} AS api-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/hire ./cmd/server \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ \
      ./cmd/migrate ./cmd/reindex ./cmd/reindex-companies \
      ./cmd/rollup-stats ./cmd/ingest ./cmd/add-board

FROM node:22-alpine AS ds-build
RUN corepack enable && corepack prepare pnpm@latest --activate
WORKDIR /design-system
COPY design-system/package.json design-system/pnpm-lock.yaml design-system/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY design-system/ ./
RUN pnpm build

FROM node:22-alpine AS web-build
RUN corepack enable && corepack prepare pnpm@latest --activate
WORKDIR /src
COPY --from=ds-build /design-system /design-system
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build && pnpm prune --prod

FROM getmeili/meilisearch:v1.49.0 AS search

FROM alpine:3.22 AS typst
ARG TARGETARCH
# The official pinned Typst archive is verified before extraction.
SHELL ["/bin/ash", "-o", "pipefail", "-c"]
# hadolint ignore=DL3018
RUN apk add --no-cache curl xz \
 && case "$TARGETARCH" in \
      amd64) target=x86_64-unknown-linux-musl; checksum=59b207df01be2dab9f13e80f73d04d7ff8273ffd46b3dd1b9eef5c60f3eeabea ;; \
      arm64) target=aarch64-unknown-linux-musl; checksum=cdf50ffc7b8ba759ed02200632eda3d78eb8b99aacb6611f4f75684990647620 ;; \
      *) echo "Unsupported architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac \
 && curl -fsSL "https://github.com/typst/typst/releases/download/v0.15.0/typst-${target}.tar.xz" -o /tmp/typst.tar.xz \
 && echo "$checksum  /tmp/typst.tar.xz" | sha256sum -c - \
 && tar -xJf /tmp/typst.tar.xz -C /tmp \
 && install -m 0755 "/tmp/typst-${target}/typst" /usr/local/bin/typst

FROM node:22-alpine
# hadolint ignore=DL3018
RUN apk add --no-cache nginx nginx-mod-http-brotli redis supervisor \
      poppler-utils ca-certificates curl libgcc \
 && mkdir -p /app /tmp/meili /tmp/nginx /tmp/redis /var/lib/nginx /var/log/nginx \
 && chown -R node:node /app /tmp/meili /tmp/nginx /tmp/redis /var/lib/nginx /var/log/nginx
WORKDIR /app
COPY --from=api-build /out/ /app/
COPY --from=api-build /src/migrations /app/migrations
COPY --from=web-build /src/build /app/build
COPY --from=web-build /src/node_modules /app/node_modules
COPY --from=web-build /src/package.json /app/package.json
COPY --from=search /bin/meilisearch /usr/local/bin/meilisearch
COPY --from=typst /usr/local/bin/typst /app/typst
COPY hosting/render/nginx.conf /app/nginx.template.conf
COPY hosting/render/supervisord.conf /app/supervisord.conf
COPY --chmod=755 hosting/render/start.sh hosting/render/boot-api.sh hosting/render/boot-web.sh hosting/render/rebuild-search.sh /app/
ENV ENV=production \
    COOKIE_SECURE=true \
    API_INTERNAL_URL=http://127.0.0.1:8080 \
    REDIS_URL=redis://127.0.0.1:6379/0 \
    MEILI_URL=http://127.0.0.1:7700 \
    MEILI_ENV=production \
    MEILI_HTTP_ADDR=127.0.0.1:7700 \
    MEILI_DB_PATH=/tmp/meili/data.ms \
    MEILI_DATA_DIR=/tmp/meili \
    MEILI_MAX_INDEXING_MEMORY=64MiB \
    MEILI_MAX_INDEXING_THREADS=1 \
    MEILI_NO_ANALYTICS=true \
    REINDEX_MIN_FREE_GB=1 \
    GOMEMLIMIT=80MiB \
    GOGC=50 \
    NODE_OPTIONS=--max-old-space-size=96 \
    TYPST_BIN=/app/typst \
    MALLOC_ARENA_MAX=2
USER node
EXPOSE 10000
ENTRYPOINT ["/app/start.sh"]
