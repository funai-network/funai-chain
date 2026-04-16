# FunAI SDK Developer Guide

This guide covers how to integrate with FunAI Chain for AI inference using the Go SDK.

## Overview

FunAI SDK (Layer 3) is the user-facing interface for sending inference requests to the decentralized AI network. It handles:

- Request signing (secp256k1)
- Streaming token reception via P2P
- Auto-retry on timeout
- Privacy (PII sanitization, TLS encryption, Tor anonymity)
- OpenAI-compatible chat API
- Function calling and JSON mode

## Installation

```bash
go get github.com/funai-network/funai-chain/sdk
```

## Quick Start

### Basic Inference

```go
package main

import (
    "context"
    "fmt"

    "github.com/funai-network/funai-chain/sdk"
)

func main() {
    client, err := sdk.NewClient(sdk.Config{
        BootPeers:  []string{"/ip4/<boot-node>/tcp/4001/p2p/<peer-id>"},
        ChainRPC:   "http://localhost:26657",
        ChainREST:  "http://localhost:1317",
        UserPubkey:  pubkeyBytes,   // secp256k1 public key
        UserPrivKey: privkeyBytes,  // secp256k1 private key (32 bytes)
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    result, err := client.Infer(context.Background(), sdk.InferParams{
        ModelId:     "qwen3-32b-q4",
        Prompt:      "Explain quantum computing in one paragraph.",
        Fee:         1_000_000, // 1 FAI
        Temperature: 7000,      // 0.7 (scale: 0-20000, where 10000 = 1.0)
        MaxTokens:   512,
        StreamMode:  true,
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(result.Output)
    fmt.Printf("Verified: %v\n", result.Verified)
}
```

### OpenAI-Compatible Chat API

```go
chat := client.NewChat()

resp, err := chat.Completions.Create(ctx, sdk.ChatCompletionRequest{
    Model: "qwen3-32b-q4",
    Messages: []sdk.Message{
        {Role: sdk.RoleSystem, Content: ptr("You are a helpful assistant.")},
        {Role: sdk.RoleUser, Content: ptr("What is blockchain?")},
    },
    Temperature: 0.7,
    MaxTokens:   256,
})
if err != nil {
    panic(err)
}

fmt.Println(*resp.Choices[0].Message.Content)

func ptr(s string) *string { return &s }
```

## Configuration

### Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ListenAddr` | string | `/ip4/0.0.0.0/tcp/0` | libp2p listen address |
| `BootPeers` | []string | — | Bootstrap peer multiaddrs |
| `KeyName` | string | — | User key name |
| `UserPubkey` | []byte | — | secp256k1 public key |
| `UserPrivKey` | []byte | — | secp256k1 private key (32 bytes) |
| `ChainRPC` | string | — | Chain RPC URL |
| `ChainREST` | string | — | Chain REST URL for queries |
| `PrivacyMode` | string | `"plain"` | `"plain"`, `"tls"`, `"tor"`, or `"full"` |
| `TorSocksAddr` | string | `127.0.0.1:9050` | Tor SOCKS5 proxy address |
| `DisableSanitization` | bool | `false` | Skip PII sanitization |
| `InferTimeout` | Duration | 30s | Inference retry timeout (5s-120s) |

## Inference Parameters

### InferParams

| Field | Type | Description |
|-------|------|-------------|
| `ModelId` | string | Model identifier or alias |
| `Prompt` | string | Input prompt text |
| `Fee` | uint64 | Maximum fee in ufai (1 FAI = 1,000,000 ufai) |
| `Temperature` | uint16 | 0 = argmax, 10000 = 1.0 (range: 0-20000) |
| `TopP` | uint16 | Nucleus sampling: 0 or 10000 = disabled, 1-9999 = threshold |
| `MaxExpire` | uint64 | Max blocks for signature validity (auto-calculated if 0) |
| `MaxTokens` | uint32 | Expected max output tokens |
| `MaxLatencyMs` | uint32 | Max first-token latency in ms (0 = no constraint) |
| `StreamMode` | bool | Request streaming response |

### InferResult

| Field | Type | Description |
|-------|------|-------------|
| `TaskId` | []byte | Unique task identifier |
| `Output` | string | Complete model output (PII restored if applicable) |
| `ResultHash` | []byte | SHA-256 of output |
| `Tokens` | []string | Individual tokens from streaming |
| `Verified` | bool | `true` if result_hash matches Worker's InferReceipt |

## OpenAI-Compatible API

The Chat API follows the OpenAI API format for easy migration.

### Chat Completion

```go
chat := client.NewChat()

resp, err := chat.Completions.Create(ctx, sdk.ChatCompletionRequest{
    Model:       "qwen3-32b-q4",
    Messages:    messages,
    Temperature: 0.7,       // OpenAI format: 0.0-2.0
    TopP:        0.9,       // OpenAI format: 0.0-1.0
    MaxTokens:   1024,
    MaxFee:      5_000_000, // 5 FAI (auto-estimated if 0)
    MaxLatencyMs: 3000,     // 3s max first-token latency
})
```

### ChatCompletionRequest Fields

| Field | Type | Description |
|-------|------|-------------|
| `Model` | string | Model identifier |
| `Messages` | []Message | Conversation history |
| `Temperature` | float64 | 0.0-2.0 (converted to 0-20000 internally) |
| `TopP` | float64 | 0.0-1.0 nucleus sampling |
| `MaxTokens` | int | Max output tokens |
| `Tools` | []Tool | Function calling tools |
| `ResponseFormat` | *ResponseFormat | `{Type: "json_object"}` for JSON mode |
| `Stream` | bool | Request streaming |
| `MaxFee` | uint64 | Max fee in ufai (auto-estimated if 0) |
| `MaxLatencyMs` | uint32 | Max first-token latency |

### Message Roles

```go
sdk.RoleSystem    = "system"
sdk.RoleUser      = "user"
sdk.RoleAssistant = "assistant"
sdk.RoleTool      = "tool"
```

## Function Calling

```go
resp, err := chat.Completions.Create(ctx, sdk.ChatCompletionRequest{
    Model:    "qwen3-32b-q4",
    Messages: messages,
    Tools: []sdk.Tool{
        {
            Type: "function",
            Function: sdk.ToolFunction{
                Name:        "get_weather",
                Description: "Get weather for a city",
                Parameters:  json.RawMessage(`{
                    "type": "object",
                    "properties": {
                        "city": {"type": "string", "description": "City name"}
                    },
                    "required": ["city"]
                }`),
            },
        },
    },
})

// Check for tool calls
if len(resp.Choices[0].Message.ToolCalls) > 0 {
    tc := resp.Choices[0].Message.ToolCalls[0]
    fmt.Printf("Call %s with %s\n", tc.Function.Name, tc.Function.Arguments)

    // Execute tool, then send result back
    messages = append(messages, resp.Choices[0].Message)
    messages = append(messages, sdk.Message{
        Role:       sdk.RoleTool,
        ToolCallID: tc.ID,
        Content:    ptr(`{"temperature": 22, "condition": "sunny"}`),
    })
    // Send follow-up request with tool result...
}
```

## JSON Mode

Force the model to return valid JSON:

```go
resp, err := chat.Completions.Create(ctx, sdk.ChatCompletionRequest{
    Model:    "qwen3-32b-q4",
    Messages: messages,
    ResponseFormat: &sdk.ResponseFormat{Type: "json_object"},
    MaxTokens: 256,
})
// resp.Choices[0].Message.Content is guaranteed valid JSON
// Auto-retries up to 3 times if model output is not valid JSON
```

## Model Discovery

```go
models := client.NewModels()
list, err := models.List(ctx)
for _, m := range list {
    fmt.Printf("%-30s  workers=%d  avg_fee=%d ufai  status=%s\n",
        m.Alias, m.ActiveWorkers, m.AvgFee, m.Status)
}
```

## Privacy Modes

| Mode | Encryption | IP Anonymity | Description |
|------|-----------|--------------|-------------|
| `plain` | No | No | Direct connection, fastest |
| `tls` | Yes (AES-256-GCM) | No | End-to-end encrypted messages |
| `tor` | No | Yes (Tor) | Routes through Tor for IP hiding |
| `full` | Yes | Yes | Tor + TLS combined |

```go
// TLS encryption (recommended)
cfg := sdk.Config{PrivacyMode: "tls", ...}

// Maximum privacy
cfg := sdk.Config{
    PrivacyMode:  "full",
    TorSocksAddr: "127.0.0.1:9050",  // requires local Tor daemon
    ...
}
```

### Encryption Details

- Algorithm: X25519 ECDH + AES-256-GCM
- Per-message forward secrecy via ephemeral X25519 keypair
- Leader's X25519 public key auto-discovered via key exchange topic
- Wire format: `[1B version][32B ephemeral_pubkey][12B nonce][ciphertext+tag]`

## PII Sanitization

By default, the SDK automatically removes PII (personally identifiable information) from prompts before sending, and restores them in the output.

Detected patterns: email, US SSN, credit card numbers, China mobile numbers, China ID numbers, US phone numbers.

```go
// Automatic (default) — no code changes needed
result, _ := client.Infer(ctx, params)
// result.Output has PII restored

// Disable sanitization
cfg := sdk.Config{DisableSanitization: true, ...}

// Manual sanitization
sr := sdk.SanitizePromptReversible("Contact john@example.com for details")
// sr.Sanitized = "Contact [PII_0] for details"
// sr.Mapping = [{Placeholder: "[PII_0]", Original: "john@example.com"}]
output := sdk.RestoreOutput(modelOutput, sr.Mapping)
```

## Fraud Verification

The SDK automatically verifies inference results (M7 verification):

1. After inference completes, SDK receives Worker's `InferReceipt` on P2P
2. Compares `result_hash` (SHA-256 of output) with Worker's signed receipt
3. If mismatch: sets `result.Verified = false` and submits `MsgFraudProof` to chain
4. FraudProof triggers: 5% stake slash + permanent tombstone of the Worker

```go
result, _ := client.Infer(ctx, params)
if !result.Verified {
    // Fraud detected — FraudProof already submitted to chain
    log.Warn("result verification failed for task", result.TaskId)
}
```

## Expiration & Timeout

- **Signature expiration**: Auto-calculated based on `MaxTokens`:
  - < 1,000 tokens: 360 blocks (30 min)
  - 1,000-10,000 tokens: 1,440 blocks (2 hours)
  - \> 10,000 tokens: 4,320 blocks (6 hours)
  - Hard cap: 17,280 blocks (24 hours)
- **Inference timeout**: Configurable via `InferTimeout` (default 30s, range 5s-120s)
- **Auto-retry**: If no streaming token received within timeout, SDK resends with same `task_id`

## Error Handling

```go
result, err := client.Infer(ctx, params)
if err != nil {
    if funaiErr, ok := err.(*sdk.FunAIError); ok {
        switch funaiErr.Code {
        case sdk.ErrInsufficientBalance:
            // Deposit more FAI
        case sdk.ErrModelNotFound:
            // Check model ID
        case sdk.ErrNoAvailableWorker:
            // No workers online for this model, retry later
        case sdk.ErrRequestTimeout:
            // All retries exhausted
        case sdk.ErrFeeTooLow:
            // Increase fee
        }
        fmt.Printf("HTTP %d: %s\n", funaiErr.HTTPStatus(), funaiErr.Message)
    }
}
```

### Error Codes

| Code | HTTP | Description |
|------|------|-------------|
| `insufficient_balance` | 402 | Inference balance too low, deposit more FAI |
| `model_not_found` | 404 | Model ID not found or not activated |
| `no_available_worker` | 503 | No workers available for this model |
| `request_timeout` | 408 | Inference timeout after retries |
| `fee_too_low` | 422 | Fee below model's minimum |
| `max_tokens_exceeded` | 400 | Requested tokens exceed model limit |
| `invalid_parameters` | 400 | Invalid request parameters |
| `network_error` | 500 | P2P network error |
| `json_parse_failed` | 422 | JSON mode: failed to get valid JSON after retries |

## Prerequisites

Before using the SDK, you need:

1. **FAI tokens** — Deposit to inference balance:
   ```bash
   funaid tx settlement deposit 10000000ufai --from mykey --chain-id funai_123123123-3
   ```

2. **Check balance**:
   ```bash
   funaid query settlement account $(funaid keys show mykey -a)
   ```

3. **Available model** — List active models to find model IDs and aliases.
