# staykey.direct: the site, web billing, email confirmation and the app link association files.
# Build from the repo root; see apps/marketing/fly.toml.
FROM node:22-bookworm-slim AS build
COPY --from=oven/bun:1.4.2 /usr/local/bin/bun /usr/local/bin/bun
ENV TURBO_TELEMETRY_DISABLED=1 NEXT_TELEMETRY_DISABLED=1
WORKDIR /repo
COPY . .
RUN bun install --frozen-lockfile --filter @staykey/marketing

# Read at build time: the home page is prerendered with prices from STAYKEY_API_URL and the store
# settings, and NEXT_PUBLIC_* and STAYKEY_ENVIRONMENT are inlined into the browser bundle.
ARG STAYKEY_ENVIRONMENT=production
ARG STAYKEY_RELEASE=dev
ARG STAYKEY_API_URL=https://api.staykey.direct
ARG STAYKEY_APP_STORE_URL=
ARG STAYKEY_PLAY_STORE_URL=
ARG STAYKEY_APP_STORE_ID=
ARG NEXT_PUBLIC_SENTRY_DSN=
ARG SENTRY_ORG=
ARG SENTRY_PROJECT=
ENV STAYKEY_ENVIRONMENT=$STAYKEY_ENVIRONMENT STAYKEY_RELEASE=$STAYKEY_RELEASE \
    STAYKEY_API_URL=$STAYKEY_API_URL STAYKEY_APP_STORE_URL=$STAYKEY_APP_STORE_URL \
    STAYKEY_PLAY_STORE_URL=$STAYKEY_PLAY_STORE_URL STAYKEY_APP_STORE_ID=$STAYKEY_APP_STORE_ID \
    NEXT_PUBLIC_SENTRY_DSN=$NEXT_PUBLIC_SENTRY_DSN SENTRY_ORG=$SENTRY_ORG SENTRY_PROJECT=$SENTRY_PROJECT
# The Sentry token uploads source maps and stays out of the image's layers.
RUN --mount=type=secret,id=SENTRY_AUTH_TOKEN \
    export SENTRY_AUTH_TOKEN=$(cat /run/secrets/SENTRY_AUTH_TOKEN 2>/dev/null || true) && \
    bun x turbo run build --filter=@staykey/marketing

FROM node:22-bookworm-slim
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
WORKDIR /app
COPY --from=build --chown=node /repo/apps/marketing/.next/standalone ./
COPY --from=build --chown=node /repo/apps/marketing/.next/static ./apps/marketing/.next/static
COPY --from=build --chown=node /repo/apps/marketing/public ./apps/marketing/public
# The social images read the DM Sans files from the working directory, which tracing can't see.
COPY --from=build --chown=node /repo/apps/marketing/assets ./apps/marketing/assets
USER node
EXPOSE 3000
CMD ["node", "apps/marketing/server.js"]
