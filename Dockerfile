FROM golang:1.25-alpine AS build
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=$GOPROXY
# -p caps parallel package compiles so the build fits in small-memory hosts.
ENV GOFLAGS="-p=2"
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/redirector ./cmd/redirector \
 && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/api /out/redirector /out/worker /usr/local/bin/
ENTRYPOINT []
