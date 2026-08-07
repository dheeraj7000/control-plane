# syntax=docker/dockerfile:1
#
# NEXT_PUBLIC_API_BASE_URL is compiled into the client bundle at build
# time (Next.js's NEXT_PUBLIC_* convention) — it can't be an ordinary
# runtime environment variable the way the Go server's config is,
# because by the time a container starts, the JavaScript shipped to
# the browser has already been bundled. Default assumes the API is
# reachable at localhost:8080 from wherever the *browser* runs (e.g.
# docker-compose publishing the server's 8080 to the host) — not the
# server container's in-network hostname, which the browser can't
# resolve.
FROM node:20-alpine AS deps
WORKDIR /app
COPY apps/dashboard/package.json apps/dashboard/package-lock.json ./
RUN npm ci

FROM node:20-alpine AS build
WORKDIR /app
ARG NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
COPY --from=deps /app/node_modules ./node_modules
COPY apps/dashboard/ ./
RUN npm run build

FROM node:20-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup -S nextjs && adduser -S nextjs -G nextjs
COPY --from=build /app/public ./public
COPY --from=build --chown=nextjs:nextjs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nextjs /app/.next/static ./.next/static
USER nextjs
EXPOSE 3000
ENTRYPOINT ["node", "server.js"]
