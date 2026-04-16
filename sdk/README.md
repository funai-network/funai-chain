# FunAI User SDK (Layer 3)

## Purpose

The SDK is the user-facing interface for inference requests.
It handles request signing, streaming, retry, and privacy.

## Directory Structure

```
sdk/
├── client.go       # Main client: sign request, send, stream, retry
├── types.go        # User-facing types
└── example/        # Example usage
    └── main.go
```

## Usage (planned)

```go
client := sdk.NewClient(sdk.Config{
    ChainRPC:   "tcp://localhost:26657",
    LeaderAddr: "/ip4/...",
    KeyName:    "user1",
})

stream, err := client.Infer(ctx, sdk.InferRequest{
    ModelId:     "sha256-of-model",
    Prompt:      "Hello, world!",
    Fee:         1_000_000, // 1 FAI
    Temperature: 7000,      // 0.7
})

for token := range stream.Tokens() {
    fmt.Print(token)
}
```

## Features (V5.2 §3, §19)

- Request signing (secp256k1, same as chain keys)
- Streaming token reception via P2P
- Auto-retry on timeout (5s no token → resend same task_id)
- Privacy: content sanitization, optional Tor, TLS encryption
- Suggested fee/timeout based on model and task size
