# Booking pages, the embed and cdn.staykey.direct/widget.js (the wildcard sends cdn. here too).
# Build from the repo root; see apps/booking/fly.toml.
FROM node:22-bookworm-slim AS build
COPY --from=oven/bun:1.4.2 /usr/local/bin/bun /usr/local/bin/bun
ENV TURBO_TELEMETRY_DISABLED=1 NEXT_TELEMETRY_DISABLED=1
WORKDIR /repo
COPY . .
RUN bun install --frozen-lockfile --filter @staykey/booking --filter @staykey/widget

# Read at build time: NEXT_PUBLIC_* and STAYKEY_ENVIRONMENT are inlined into the browser bundle.
ARG STAYKEY_ENVIRONMENT=production
ARG STAYKEY_RELEASE=dev
ARG NEXT_PUBLIC_ROOT_DOMAIN=staykey.direct
ARG NEXT_PUBLIC_SENTRY_DSN=
ARG SENTRY_ORG=
ARG SENTRY_PROJECT=
ENV STAYKEY_ENVIRONMENT=$STAYKEY_ENVIRONMENT STAYKEY_RELEASE=$STAYKEY_RELEASE \
    NEXT_PUBLIC_ROOT_DOMAIN=$NEXT_PUBLIC_ROOT_DOMAIN NEXT_PUBLIC_SENTRY_DSN=$NEXT_PUBLIC_SENTRY_DSN \
    SENTRY_ORG=$SENTRY_ORG SENTRY_PROJECT=$SENTRY_PROJECT
# The Sentry token uploads source maps and stays out of the image's layers.
RUN --mount=type=secret,id=SENTRY_AUTH_TOKEN \
    export SENTRY_AUTH_TOKEN=$(cat /run/secrets/SENTRY_AUTH_TOKEN 2>/dev/null || true) && \
    bun x turbo run build --filter=@staykey/booking --filter=@staykey/widget

FROM node:22-bookworm-slim
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3001 HOSTNAME=0.0.0.0
WORKDIR /app
COPY --from=build --chown=node /repo/apps/booking/.next/standalone ./
COPY --from=build --chown=node /repo/apps/booking/.next/static ./apps/booking/.next/static
COPY --from=build --chown=node /repo/packages/widget/dist/widget.js ./apps/booking/public/widget.js
USER node
EXPOSE 3001
CMD ["node", "apps/booking/server.js"]
