# Stage 1: Build
FROM golang:1.25-bookworm AS builder

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build both binaries
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -mod=readonly -ldflags "-s -w -extldflags '-static'" \
    -o /out/funaid ./cmd/funaid
RUN CGO_ENABLED=1 GOOS=linux go build -mod=readonly -ldflags "-s -w -extldflags '-static'" \
    -o /out/funai-node ./cmd/funai-node

# Stage 2: Chain node image
FROM debian:bookworm-slim AS funaid
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl python3 && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/funaid /usr/local/bin/funaid
EXPOSE 26656 26657 1317 9090
ENTRYPOINT ["funaid"]
CMD ["start"]

# Stage 3: P2P node image
FROM debian:bookworm-slim AS funai-node
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/funai-node /usr/local/bin/funai-node
EXPOSE 4001 9091
ENTRYPOINT ["funai-node"]
