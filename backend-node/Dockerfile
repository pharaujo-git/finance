# Install and compile in one stage so the runtime image carries no toolchain.
FROM node:24-slim AS build

WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci

COPY tsconfig.json tsconfig.build.json ./
COPY src ./src
RUN npm run build && npm prune --omit=dev


FROM node:24-slim

ENV NODE_ENV=production \
    PORT=8083

WORKDIR /app
COPY --from=build /src/node_modules ./node_modules
COPY --from=build /src/dist ./dist
COPY package.json ./

# The base image ships an unprivileged `node` user; nothing here writes to disk.
USER node

EXPOSE 8083

# The shell form is what expands $PORT, which the platform sets.
CMD ["sh", "-c", "exec node dist/main.js"]
