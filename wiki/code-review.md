# Code vs Spec Compliance

Tracking spec-vs-implementation gaps identified in the funai-chain code review. Baseline commit: `38bc1ff`. Source: [funai-chain-review.md](../docs/funai-chain-review.md)

## Previously Fixed (11 of 13)

| ID | Description | Status |
|----|-------------|--------|
| P0-1 | FraudProof tombstone | FIXED |
| P0-3 | ChaCha20 2^64 | FIXED |
| P0-4 | Audit VRF seed | FIXED |
| P0-5 | X25519 key | FIXED |
| P0-6 | Key exchange sig | PARTIALLY FIXED |
| P0-7 | `jailAuditors` | FIXED |
| P0-8 | `expire_block` | FIXED |
| P0-9 | FraudProof receipt | FIXED |
| P0-10 | PII Chinese patterns | FIXED |
| P1-1 | Re-audit timeout | FIXED |
| P1-2 | Audit fund FAIL | FIXED |
| P1-3 | Softmax order | FIXED |
| P1-5 | Leader sig scope | FIXED |

## OPEN P0 -- Critical Blockers

These must be resolved before any testnet with `temperature > 0` traffic.

### P0-1: Sampling uses logprob instead of raw logits (double softmax)

Worker returns log-probabilities; the verification path applies softmax again, producing a double-softmax distribution. When `T != 1.0` this causes mass false FAILs. **Most critical open issue.**

### P0-2: Worker uses TGI native sampling instead of ChaCha20

The spec requires deterministic ChaCha20-based sampling (RFC 8439) so verifiers can reproduce the exact token sequence. The Worker currently delegates sampling to TGI's built-in sampler, which is non-deterministic. Result: guaranteed verification failure when `temp > 0`.

### P0-3: SDK key exchange signature verification is a no-op

The SDK accepts the key exchange response without actually verifying the signature. An attacker on the network path can substitute their own key and read/modify inference traffic (MITM).

## OPEN P1 -- Severe

| ID | Description |
|----|-------------|
| P1-1 | VRF keeper uses bech32 as pubkey (fallback) -- causes chain/P2P VRF mismatch |
| P1-2 | `LogitsHash` uses placeholder zeros -- auditors cannot verify logits integrity |
| P1-3 | `AssignTask` missing `Temperature`, `UserSeed`, `DispatchBlockHash` -- allows MITM parameter tamper |
| P1-4 | Leader `PrivKey` never set -- `LeaderSig` is always empty on every dispatched task |
| P1-5 | `SelectVerifiersForTask` seed missing `result_hash` -- verifier selection does not match spec |

## P2 -- Moderate (12 issues)

Twelve moderate issues covering edge cases in timeout handling, metric reporting, retry logic, and parameter validation. See [funai-chain-review.md](../docs/funai-chain-review.md) for the full list.

## P3 -- Low (4 issues)

Four low-severity issues related to logging verbosity, documentation gaps, and cosmetic inconsistencies. See [funai-chain-review.md](../docs/funai-chain-review.md) for details.

## Priority Summary

**Most urgent:** P0-1 + P0-2 together mean that any task with `temperature > 0` will fail verification 100% of the time. P1-4 (unsigned `AssignTask`) leaves leader dispatch unauthenticated.

## Related Pages

- [Security Audit Findings](security-audit.md)
- [Test Plan Status](test-status.md)
- [Verification](verification.md)
- [VRF](vrf.md)
