FROM golang:1.16-alpine as build

# Maintainer information
LABEL Maintainer="Jeffrey Yong"

# Arguments Variables (custom)
ARG COMMIT='local'
ARG VERSION='v0.0.0'

WORKDIR /build

COPY . .

# Setup
RUN apk update && \
    apk --no-cache add ca-certificates

# Build application
RUN apk --no-cache add ca-certificates && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build \
    -mod vendor \
    -o /app ./cmd/server

# Build scratch image
FROM scratch

# Copy certificates
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=build /app /
COPY --from=build /build/migrations /migrations
COPY config.yaml /

ENTRYPOINT ["/app"]
