# Verification Protocol

FunAI Chain uses a multi-stage verification protocol to ensure inference correctness without re-executing full inference. All verification happens off-chain over the [P2P layer](../p2p/), with results packaged on-chain by Proposers.

## Teacher Forcing

After a Worker completes inference, it computes a VRF verifier ranking using the [unified VRF formula](../x/vrf/) with `alpha = 0.5` and `seed = task_id || result_hash`. The executing Worker is excluded from the verifier pool.

The Worker sends the prompt and complete output to the **top 3** ranked verifiers. Each verifier runs a single forward pass (~0.6s) using teacher forcing -- feeding the prompt plus the Worker's full output through the model in one pass and extracting logits at checkpoint positions.

## Logits Check (temperature = 0)

For deterministic inference (`temperature = 0`):

1. **5 VRF-selected positions** within the output are checked
2. At each position, the verifier compares its logits against the Worker's logits
3. If the absolute difference is less than **epsilon** (model-specific tolerance from [Model Registry](../x/modelreg/)), the position is a **match**
4. **4/5 match = PASS**, 3/5 or fewer = **FAIL**

The 4/5 threshold (rather than 5/5) reduces false positives by approximately **500x**, accounting for minor floating-point divergences across hardware.

## Deterministic Sampling Verification (temperature > 0)

When `temperature > 0`, verifiers must also reproduce the sampling decisions. This requires a fully deterministic sampling pipeline.

### Seed Derivation

```
final_seed = SHA256(user_seed || dispatch_block_hash || task_id)
```

### PRNG: ChaCha20 (RFC 8439)

For each token position:

- **Key**: `final_seed[0:32]` (32 bytes)
- **Nonce**: `uint64_le(position)` padded to 12 bytes
- **Counter**: `0`

### Random Number Extraction

1. Take the first 8 bytes of ChaCha20 output
2. Interpret as `uint64` (little-endian)
3. Convert to `float64`
4. Divide by `2^64`
5. Result is a uniform random value in `[0, 1)`

### Sampling Pipeline

All intermediate math uses **float32** precision:

| Step | Precision | Details |
|------|-----------|---------|
| Temperature scaling | float32 | Divide logits by temperature |
| Softmax | float32 | `expf()`, accumulation order strictly `0` to `vocab_size - 1` |
| Sampling accumulation | float32 | CDF accumulation in float32 |
| Threshold comparison | float64 | Compare accumulated CDF against random value |

**Critical constraints:**
- ALL arithmetic operations use **float32** (not float64) for cross-implementation consistency
- Softmax accumulation order is strictly `token_id 0` to `vocab_size - 1` -- any other order produces different rounding and breaks determinism
- V1 only supports **temperature** as a sampling parameter (`uint16`, `10000 = 1.0`, valid range `0-20000`). No top-p, top-k, or penalty parameters.

## Combined Judgment (temperature > 0)

Verification proceeds in two phases:

### Pre-checks

1. **Hash match**: verify the output hash matches what the Worker published
2. **Self-consistency**: verifier's own sampling reproduces the same tokens
3. **Seed verification**: confirm `final_seed` derivation is correct

### Per-position Scoring

Each of the 5 checked positions receives one of three labels:

- **Match**: both logits and sampled token agree
- **Mismatch**: logits or sampled token disagree beyond tolerance
- **Exemption**: position excluded due to known non-determinism edge cases

### Final Decision

| Total mismatches | Result |
|-----------------|--------|
| 0-2 | **PASS** |
| 3-5 | **FAIL** |

## Aggregation

- **3/3 verifiers PASS** -> task status = `VERIFIED(SUCCESS)`
- **Any verifier reports FAIL** -> task status = `VERIFIED(FAIL)`

After verification, the task enters the [settlement state machine](settlement.md): 90% are directly `CLEARED` for [batch settlement](../x/settlement/), while 10% proceed to `PENDING_AUDIT`.

## Verifier Fallback

Verifiers ranked **4 through 8** passively monitor the P2P topic. If fewer than 3 verification results arrive within **2 seconds**, fallback verifiers proactively step in to fill the gap.

If no quorum is reached after **30 seconds**, the Worker recalculates the VRF ranking using a new `block_height`, producing a fresh set of candidate verifiers.

## Sources

- [FunAI V52 Design Specification](../docs/FunAI_V52_Final.md)
- [VRF Module](../x/vrf/)
- [Settlement Module](../x/settlement/)
- [Jail & Slashing Mechanism](jail-and-slashing.md)
