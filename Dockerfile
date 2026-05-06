# Stage 1: build
FROM golang:1.23-alpine AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /typesense-snapshot-controller ./cmd/...

# Stage 2: runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /
COPY --from=builder /typesense-snapshot-controller /typesense-snapshot-controller
USER 65534:65534
ENTRYPOINT ["/typesense-snapshot-controller"]
