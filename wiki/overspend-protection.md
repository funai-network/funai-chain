# Overspend Protection

FunAI Chain uses a three-layer defense to prevent users from spending more than their deposited balance. Because inference happens off-chain on the [P2P layer](p2p-layer.md) but payment settles on-chain via [BatchSettlement](settlement.md), there is an inherent delay between spending and deduction. These three layers close that gap.

Sources: [FunAI V52 Final Design Spec](../docs/FunAI_V52_Final.md), [S9 Per-Token Billing Supplement](../docs/S9_PerToken_Billing_Supplement.md)

## Layer 1 -- Leader Local Tracking (Soft)

The Leader maintains a real-time estimate of each user's available balance:

```
available = on_chain_balance - local_pending_total
```

When a new inference request arrives, the Leader checks:

```
request_cost <= available  -->  accept
request_cost >  available  -->  reject
```

This is a **soft** check -- the Leader's view may be slightly stale (on-chain balance refreshes periodically), but it catches the vast majority of overspend attempts before they reach a Worker.

## Layer 2 -- Worker Self-Protection

Before accepting a dispatched task, the Worker independently checks the user's on-chain balance:

```
balance < fee * 3  -->  reject
```

The **3x safety factor** accounts for in-flight tasks that have not yet settled. If the user's balance cannot cover 3x the task fee, the Worker refuses the task and it falls to the next VRF rank (see [P2P dispatch](p2p-layer.md)).

## Layer 3 -- On-Chain Fallback

The `MsgBatchSettlement` handler in the [`x/settlement/`](../x/settlement/) module processes entries **one by one**. If a user's on-chain balance is insufficient to cover an entry:

- That entry's status is set to **REFUNDED**.
- The entry is skipped -- no payment to Worker or verifiers.
- **Remaining entries in the batch continue processing normally.**

This guarantees that overspend never causes a batch to fail entirely, and no Worker is paid with funds that do not exist.

## Why Overspend Is Not Worthwhile

Even if a user manages to slip a few tasks past Layers 1 and 2, the outcome is not profitable:

- The user's balance is **fully depleted** by the on-chain settlement.
- To submit more requests, the user must **deposit again** -- an on-chain transaction with gas costs.
- The sustained cost of repeatedly depositing small amounts and burning through them exceeds any benefit from the free inference obtained.

## Per-Token Billing (Shadow Balance)

Per-token billing (described in the [S9 supplement](../docs/S9_PerToken_Billing_Supplement.md)) adds a finer-grained tracking mechanism:

- The Leader maintains a **`pending_fees` map** per user, refreshed every **5 seconds** from on-chain state.
- Cross-model Leaders do **NOT** synchronize their pending_fees maps. Each Leader tracks only the models it dispatches.
- The [settlement](settlement.md) layer acts as the final authority -- it **caps payouts at the user's actual on-chain balance**, regardless of what any Leader estimated.

### SDK EstimateFee

The client SDK provides upfront fee estimation:

```
max_fee = input_tokens * input_price + max_tokens * output_price
```

This estimate is attached to the inference request and used by the Leader for Layer 1 tracking.

### Worker Local Truncation

During streaming inference, the Worker tracks cumulative cost in real time:

```
budgetLimit = max_fee * 95%
```

If `running_cost` reaches `budgetLimit`, the Worker **truncates generation** -- stopping output before the user's budget is exceeded. The 5% buffer ensures the final cost stays within `max_fee` even after accounting for rounding and overhead.

## Summary

| Layer | Where | Check | Timing |
|-------|-------|-------|--------|
| 1 | Leader | `available = on_chain_balance - local_pending_total` | Before dispatch |
| 2 | Worker | `balance < fee * 3x` | Before accepting task |
| 3 | On-chain | Insufficient balance --> REFUNDED | During BatchSettlement |

## Related Pages

- [P2P layer](p2p-layer.md) -- dispatch and Leader mechanics
- [Settlement state machine](settlement.md) -- how BatchSettlement processes entries
- [Architecture overview](architecture.md) -- chain as bank, inference off-chain
