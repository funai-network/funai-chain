# Token Economics

FunAI Chain's native token **$FAI** powers all economic activity: inference payments, worker staking, block rewards, and governance.

## Token Basics

| Parameter | Value |
|-----------|-------|
| Token symbol | $FAI |
| On-chain denom | `ufai` |
| Decimal conversion | 1 FAI = 1,000,000 ufai |
| Total supply | 210 billion FAI |
| Block time | 5 seconds |
| Epoch length | 100 blocks (500 seconds) |

## Block Rewards

| Parameter | Value |
|-----------|-------|
| Reward per block | 4,000 FAI |
| Halving interval | 26,250,000 blocks (~4.16 years) |

Rewards are calculated once per **epoch end** (every 100 blocks).

## Staking Requirements

| Parameter | Value |
|-----------|-------|
| Minimum worker stake | 10,000 FAI (governance adjustable) |
| Worker exit waiting period | 21 days |
| Cold start period | First 3 days: free registration, no stake required |

## Reward Distribution

### With Inference Activity

When the network is processing inference tasks:

| Split | Recipients | Calculation |
|-------|-----------|-------------|
| **99%** | Workers (by inference contribution) | `w_i = 0.8 * (fee_i / sum_fee) + 0.2 * (count_i / sum_count)` |
| **1%** | Verifiers & Auditors (by verification/audit count) | Proportional to completed verifications and audits |

Only tasks in `CLEARED` status are counted toward reward distribution. See the [settlement state machine](../x/settlement/) for task lifecycle details.

### Without Inference Activity

When no inference has occurred during the epoch:

| Split | Recipients | Calculation |
|-------|-----------|-------------|
| **100%** | Consensus committee (100 validators) | Proportional to signed blocks |

### Why 1% for Verification?

Block rewards are approximately **69x larger** than inference fees. Without the 1% verification split:

- Verification **loses money**: -0.033 FAI per verification task
- Workers would avoid verification duty

With the 1% split:

| Role | Earnings |
|------|----------|
| Verification | 0.61 FAI/s |
| Inference | 0.546 FAI/s |

Verification becomes **12% more profitable** than inference, ensuring sufficient verifier participation.

## Fee Distribution per Task

### SUCCESS (task passes verification)

| Recipient | Share | Notes |
|-----------|-------|-------|
| Worker | 95% | Executing worker |
| 3 Verifiers | 4.5% (1.5% each) | [Verification protocol](verification.md) |
| Audit fund | 0.5% | Funds random [audits](../x/settlement/) |

The user pays 100% of the agreed fee.

### FAIL (task fails verification)

| Recipient | Share | Notes |
|-----------|-------|-------|
| Worker | 0% | Worker is [jailed](jail-and-slashing.md) |
| 3 Verifiers | 4.5% (1.5% each) | Compensated for verification work |
| Audit fund | 0.5% | Funds random audits |

The user pays only **5%** of the original fee.

## User Deposits

Users pre-deposit FAI into their inference balance via `MsgDeposit`. Withdrawals use `MsgWithdraw`. The [settlement module](../x/settlement/) handles balance tracking and batch payouts.

Three layers of [overspend protection](../p2p/leader/) prevent users from spending beyond their balance:

1. Leader local tracking of pending totals
2. Worker self-check with 3x safety factor
3. On-chain fallback during `MsgBatchSettlement`

## Sources

- [FunAI V52 Design Specification](../docs/FunAI_V52_Final.md)
- [Reward Module](../x/reward/)
- [Settlement Module](../x/settlement/)
- [Worker Module](../x/worker/)
