# The build stage runs on the build machine's native architecture: every
# dependency in package.json is pure JavaScript, so the build/ output and
# node_modules are portable across CPU architectures and no emulation is
# needed for the linux/arm64 image.

FROM --platform=$BUILDPLATFORM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:24-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/build ./build
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/package.json ./package.json
COPY --from=build /app/package-lock.json ./package-lock.json
EXPOSE 3000
CMD ["node", "build"]
