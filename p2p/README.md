# FunAI P2P Layer (Layer 2)

## Architecture

The P2P layer handles all real-time inference operations using libp2p.
The chain (Layer 1) only handles deposits, settlements, and bookkeeping.

## Directory Structure

```
p2p/
├── host/           # libp2p host setup, peer discovery, topic management
│   └── host.go
├── types/          # P2P message types (InferRequest, InferReceipt, VerifyResult)
│   └── messages.go
├── leader/         # Leader node: dispatch, VRF ranking, busy tracking
│   └── leader.go
├── worker/         # Worker node: accept tasks, run inference, push to verifiers
│   └── worker.go
├── verifier/       # Verifier: teacher forcing, logits comparison, signing
│   └── verifier.go
├── proposer/       # Proposer: collect evidence, construct BatchSettlement
│   └── proposer.go
└── node.go         # FunAI P2P node (combines all roles)
```

## Node Roles (V5.2 §4.2)

Every registered Worker runs all 7 roles simultaneously:
1. **P2P Node** — always on, relay messages, maintain routing
2. **Inference Executor** — GPU runs model when assigned
3. **Verifier** — teacher forcing when selected by VRF
4. **Auditor** — random check when selected by VRF
5. **Leader** — dispatch tasks for assigned model_id (30s rotation)
6. **Proposer** — package BatchSettlement (CometBFT rotation)
7. **Validator** — consensus signing (100-member committee, 10min rotation)

## Message Flow

```
User SDK → [InferRequest] → Leader
Leader → [AssignTask] → Worker (VRF rank #1)
Worker → [stream tokens] → User SDK
Worker → [InferReceipt + prompt + output] → Verifiers (top 3 by VRF)
Verifiers → [VerifyResult] → P2P broadcast
Worker collects 3 PASS → complete evidence
Proposer packages → MsgBatchSettlement → Chain
```

## Tech Stack

- **P2P**: go-libp2p (topic-based pub/sub by model_id)
- **Inference**: HTTP/gRPC to local vLLM/TGI instance
- **Signing**: secp256k1 (same keys as chain)
- **Serialization**: JSON (matching chain types)
