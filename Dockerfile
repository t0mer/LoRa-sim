# syntax=docker/dockerfile:1

# Stage 1: build the SPA into the Go embed directory.
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend
WORKDIR /build
COPY web/package*.json web/
RUN cd web && npm ci
COPY web/ web/
RUN cd web && npm run build   # writes internal/webui/dist

# Stage 2: cross-compile the Go binary (CGO-free) with the embedded SPA.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev
ENV GOTOOLCHAIN=local
RUN apk add --no-cache ca-certificates
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath \
    -ldflags="-s -w -X github.com/t0mer/cylon/internal/version.Version=${VERSION}" \
    -o /cylon ./cmd/cylon

# Stage 3: minimal scratch image with CA certs for outbound TLS to AWS.
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /cylon /cylon
# 8080: UI/API + /ws · 6000: tag TCP · 9100: metrics
EXPOSE 8080 6000 9100
VOLUME ["/var/lib/cylon", "/etc/cylon/creds"]
ENTRYPOINT ["/cylon"]
CMD ["serve"]
