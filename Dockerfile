# syntax=docker/dockerfile:1

# Build stage: cross-compile natively per target platform (CGO-free).
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
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath \
    -ldflags="-s -w -X github.com/t0mer/cylon/internal/version.Version=${VERSION}" \
    -o /cylon ./cmd/cylon

# Runtime stage: minimal scratch image with CA certs for outbound TLS to AWS.
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /cylon /cylon
# 8080: UI/API + /ws · 6000: tag TCP · 9100: metrics
EXPOSE 8080 6000 9100
VOLUME ["/var/lib/cylon", "/etc/cylon/creds"]
ENTRYPOINT ["/cylon"]
CMD ["serve"]
