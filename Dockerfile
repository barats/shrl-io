# One Dockerfile, five images. The shared build stage cross-compiles the Go
# services on the build machine's native architecture (no emulation needed);
# the per-service targets below are the image roots, selected via --target
# (or docker-bake.hcl for the release matrix).

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
# -p caps parallel package compiles so the build fits in small-memory hosts.
ENV GOFLAGS="-p=2"
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/api ./api \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/auth ./auth \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/redirector ./redirector \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/worker ./worker

FROM alpine:3.20 AS base
RUN apk add --no-cache ca-certificates tzdata

FROM base AS api
COPY --from=build /out/api /usr/local/bin/api
ENTRYPOINT ["/usr/local/bin/api"]

FROM base AS auth
COPY --from=build /out/auth /usr/local/bin/auth
ENTRYPOINT ["/usr/local/bin/auth"]

FROM base AS redirector
COPY --from=build /out/redirector /usr/local/bin/redirector
ENTRYPOINT ["/usr/local/bin/redirector"]

FROM base AS worker
COPY --from=build /out/worker /usr/local/bin/worker
ENTRYPOINT ["/usr/local/bin/worker"]
