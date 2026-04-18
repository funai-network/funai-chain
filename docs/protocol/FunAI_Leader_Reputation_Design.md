# FunAI Leader Reputation Mechanism Design

**Date:** 2026-04-18
**Priority:** P2 (non-blocking for launch; first iteration batch post-launch)
**Effort estimate:** 200–300 lines of code across 3 modules

---

## Problem

Leader election uses α=1.0 (pure stake weight). Nodes with large stake are likely to be elected Leader repeatedly. If such a node is idle, slow to respond, or dispatches out of order, the 30-second epoch is short but the situation recurs. Currently there is no independent performance evaluation for Leaders — whether they perform well or poorly, selection is purely stake-based.

## Design

### New Fields

`x/worker/types/worker.go` — add to the Worker struct:

```go
// Leader reputation score, independent of inference reputation ReputationScore.
// Range 0-12000 (i.e. 0.0-1.2), initial value 10000 (1.0).
// Folded into the Leader VRF election formula; nodes with low reputation rank lower even with high stake.
LeaderReputationScore uint32 `protobuf:"varint,25,opt,name=leader_reputation_score,proto3" json:"leader_reputation_score"`

// Leader consecutive failover counter. Resets to zero on a normal epoch.
ConsecutiveLeaderFailovers uint32 `protobuf:"varint,26,opt,name=consecutive_leader_failovers,proto3" json:"consecutive_leader_failovers"`
```

Constants (`x/worker/types/worker.go`):

```go
LeaderRepInitial          uint32 = 10000 // 1.0
LeaderRepMax              uint32 = 12000 // 1.2
LeaderRepIdlePenalty      uint32 = 1000  // -0.1 (0 dispatches within epoch)
LeaderRepFailoverPenalty  uint32 = 500   // -0.05 (consecutive failover)
LeaderRepFailoverLimit    uint32 = 3     // 3 consecutive failovers trigger deduction
LeaderRepSkipPenalty      uint32 = 2000  // -0.2 (illegal skip of VRF ranking)
LeaderRepGoodEpoch        uint32 = 100   // +0.01 (reward for normal epoch)
LeaderRepDecayStep        uint32 = 50    // ±0.005 (hourly decay toward 1.0)
```

### Leader VRF Formula Change

In `RankWorkers` in `x/vrf/types/vrf.go`, when alpha is for Leader election:

```
score = hash(seed || pubkey) / (stake × leader_reputation × speed)
```

This integrates in exactly the same way as the existing inference reputation and speed factor — folded into effective_stake.

Specific change: add a `LeaderReputation float64` field to the `RankedWorker` struct. `RankWorkers` selects `LeaderReputation` or `Reputation` based on the calling context (Leader election vs. Worker dispatch).

Simplest implementation: add a new `AlphaLeader VRFAlpha = 1.0`, numerically equal to `AlphaDispatch`, but `RankWorkers` uses `LeaderReputation` for `AlphaLeader` and `Reputation` for `AlphaDispatch`.

---

## Three Automatic Detection Scenarios

All handled automatically by on-chain keepers — no Worker reporting required, no new Msg types.

### Scenario 1: Leader Dispatches 0 Tasks Within an Epoch

**Detection location:** `x/vrf/keeper/keeper.go` → `CheckLeaderTimeouts` (already called by EndBlocker)

**Detection logic:**

```go
// When an epoch expires, check whether this Leader dispatched any settlement entries during its term.
func (k Keeper) CheckLeaderPerformance(ctx sdk.Context, modelId string, leader LeaderInfo) {
    // Query the number of settlement entries from this Leader between StartBlock and EndBlock.
    entryCount := settlementKeeper.CountEntriesByLeader(ctx, leader.Address, leader.StartBlock, leader.EndBlock)
    
    if entryCount == 0 {
        // There were user requests this epoch but Leader dispatched 0 tasks.
        // Check whether the model had any user activity (optional: no deduction if the entire model had no requests).
        workerKeeper.DeductLeaderReputation(ctx, leader.Address, LeaderRepIdlePenalty)
    } else {
        // Normal operation: add score + reset failover counter.
        workerKeeper.AddLeaderReputation(ctx, leader.Address, LeaderRepGoodEpoch)
        workerKeeper.ResetLeaderFailovers(ctx, leader.Address)
    }
}
```

**Prerequisite:** `SettlementEntry` must record `LeaderAddress`. This field does not currently exist in the entry; it must be written by `p2p/proposer` when packaging. The Leader address can be recovered from `AssignTask.LeaderSig` (public key recovery from signature).

**Change scope:**
- `x/settlement/types/` — add `LeaderAddress` field to SettlementEntry
- `p2p/proposer/proposer.go` — write Leader address when packaging entries
- `x/vrf/keeper/keeper.go` — extend CheckLeaderTimeouts

### Scenario 2: Repeated Failover Triggers

**Detection location:** `x/vrf/keeper/keeper.go` → `CheckLeaderTimeouts` (existing timeout detection)

**Detection logic:**

```go
// Existing code: detect Leader timeout and re-elect.
if currentHeight - leader.LastHeartbeat > params.LeaderTimeoutBlocks {
    // Existing: trigger re-election.
    k.SelectLeader(ctx, modelId, workers)
    
    // New: record failover.
    w := workerKeeper.GetWorker(ctx, leader.Address)
    w.ConsecutiveLeaderFailovers++
    
    if w.ConsecutiveLeaderFailovers >= LeaderRepFailoverLimit {
        workerKeeper.DeductLeaderReputation(ctx, leader.Address, LeaderRepFailoverPenalty)
        w.ConsecutiveLeaderFailovers = 0  // reset to zero after deduction, start counting again
    }
    
    workerKeeper.SetWorker(ctx, w)
}
```

**Change scope:** Only a few lines in `x/vrf/keeper/keeper.go`.

### Scenario 3: Illegal Dispatch (Skipping VRF Ranking)

**Detection location:** `x/settlement/keeper/keeper.go` → `ProcessBatchSettlement`

**Detection logic:**

```go
// When processing each settlement entry.
func (k Keeper) verifyDispatchFairness(ctx sdk.Context, entry SettlementEntry) {
    // Recompute VRF ranking using task_id + dispatch_block_hash from the entry.
    seed := sha256(entry.TaskId + entry.DispatchBlockHash)
    
    // Get the list of Workers online at that time (queryable from on-chain state).
    workers := vrfKeeper.GetOnlineWorkersAtBlock(ctx, entry.ModelId)
    
    // Recompute ranking.
    ranked := vrf.RankWorkers(seed, workers, AlphaDispatch)
    
    // Find the actual executor's rank.
    actualRank := -1
    for i, w := range ranked {
        if w.Address == entry.WorkerAddress {
            actualRank = i + 1
            break
        }
    }
    
    // If rank > 3 and the nodes ahead were online at the time (no legitimate timeout/rejection reason).
    if actualRank > 3 {
        // Check whether rank#1/#2/#3 have rejection records.
        // If no rejection records → Leader illegally skipped ranking.
        workerKeeper.DeductLeaderReputation(ctx, entry.LeaderAddress, LeaderRepSkipPenalty)
    }
}
```

**Precision caveat:** This detection has one important note — when the Leader dispatched, rank#1 may genuinely have timed out (no acceptance within 1 second), so the Leader legitimately fell back to rank#2. This timeout occurs at the P2P layer and is invisible on-chain. Therefore on-chain detection can only catch obvious violations where rank > 3 (since fallback is limited to 3 ranks). Execution by rank#2 or rank#3 is treated as legitimate.

**Change scope:**
- `x/settlement/keeper/keeper.go` — add verification inside ProcessBatchSettlement
- `x/vrf/keeper/keeper.go` — expose RankWorkers for settlement keeper to call

---

## Reputation Recovery

Symmetric with inference reputation:

```go
// Normal Leader epoch: +0.01
func (k Keeper) AddLeaderReputation(ctx sdk.Context, addr string, delta uint32) {
    w := k.GetWorker(ctx, addr)
    w.LeaderReputationScore += delta
    if w.LeaderReputationScore > LeaderRepMax {
        w.LeaderReputationScore = LeaderRepMax
    }
    k.SetWorker(ctx, w)
}

// Hourly decay toward 1.0 (BeginBlocker, done together with inference reputation)
func (k Keeper) LeaderReputationDecay(ctx sdk.Context) {
    // Executes every 720 blocks (1 hour).
    workers := k.GetAllWorkers(ctx)
    for _, w := range workers {
        if w.LeaderReputationScore > LeaderRepInitial {
            w.LeaderReputationScore -= LeaderRepDecayStep
        } else if w.LeaderReputationScore < LeaderRepInitial {
            w.LeaderReputationScore += LeaderRepDecayStep
        }
        k.SetWorker(ctx, w)
    }
}
```

---

## Effect

Using the earlier example: whale node at 100x stake, normal node at 1x stake.

**Without Leader reputation:** whale Leader probability ≈ 50%

**After Leader reputation drops to 0.5:** effective weight = 100 × 0.5 = 50, other nodes = 1 × 1.0. Whale Leader probability drops to ≈ 34%.

**After Leader reputation drops to 0.1:** effective weight = 100 × 0.1 = 10. Whale Leader probability drops to ≈ 9%. Same as an ordinary 10x node.

10 consecutive idle epochs drops reputation from 1.0 to 0.0 — completely excluded from Leader candidacy.

---

## Implementation Order

| Phase | Content | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | Add fields + constants to Worker struct | 20 lines | None |
| 2 | Scenario 2 (failover deduction) | 30 lines | Phase 1 |
| 3 | Scenario 1 (0-dispatch deduction) + add LeaderAddress to SettlementEntry | 80 lines | Phase 1 |
| 4 | Integrate LeaderReputationScore into Leader VRF formula | 40 lines | Phase 1 |
| 5 | Reputation recovery + hourly decay | 30 lines | Phase 1 |
| 6 | Scenario 3 (illegal dispatch detection) | 100 lines | Phase 3 |
| 7 | Tests | 100 lines | All |

Phases 1–5 total approximately 200 lines and can ship first. Phase 6 is more complex (requires cross-module VRF recomputation call) and can be a separate PR later.

---

## Relationship to Existing Mechanisms

| Existing mechanism | Leader reputation | Relationship |
|--------------------|-------------------|--------------|
| ReputationScore (inference reputation) | LeaderReputationScore | Independent. One governs inference quality, the other governs dispatch quality. |
| ConsecutiveRejects (consecutive rejections) | ConsecutiveLeaderFailovers | Symmetric design. One governs Worker refusal to accept tasks, the other governs Leader idleness. |
| Jail mechanism | No Leader jail needed | The consequence of an idle Leader is rank demotion so it cannot be elected Leader; jail is not needed. |
| VRF AlphaDispatch | New AlphaLeader (or reuse α=1.0) | Leader election and Worker dispatch use the same α but different reputation scores. |

---

## Review Notes (pre-implementation)

Findings from the 2026-04-18 design review. Kept separate from the body
above so the original intent is preserved; every item needs to be handled
or explicitly waived by the implementation PR.

### Must fix before coding

1. **Proto tag collision.** The struct snippet uses `varint,25` for
   `LeaderReputationScore` and `26` for `ConsecutiveLeaderFailovers`. Tag
   25 is already occupied by `AvgLatencyMs` (introduced by PR #10). The
   implementation PR must use tags 26 and 27.

2. **Missing `EffectiveLeaderReputation()` helper.** Go's zero-value for
   `uint32` is 0, which under the proposed formula multiplies
   `effective_stake` by 0. Any Worker registered before the upgrade
   would be permanently ineligible for Leader election. The helper must
   treat `LeaderReputationScore == 0` as 1.0 (same contract as the
   existing `EffectiveReputation()`).

3. **Genesis migration on upgrade.** Beyond the helper, existing Worker
   rows need a one-shot migration that sets `LeaderReputationScore =
   LeaderRepInitial` when the new field is first materialized.

### Design ambiguities to resolve

4. **Scenario 1 + Scenario 2 double-penalty.** A failover epoch
   necessarily has ~0 dispatches. As written, a single failure can
   trigger both `LeaderRepFailoverPenalty` (−0.05 once 3-strike hits)
   and `LeaderRepIdlePenalty` (−0.1 every epoch). Intentional? If not,
   `CheckLeaderPerformance` should skip the idle branch for epochs that
   terminated via failover (`leader.EndBlock < leader.StartBlock +
   LeaderEpochDuration`).

5. **Scenario 3 pseudocode vs comment disagreement.** The pseudocode
   deducts unconditionally when `actualRank > 3`; the adjacent comment
   says "check whether rank#1/#2/#3 have rejection records first".
   Under current P2P failover bounds (up to 3 ranks) the unconditional
   form is defensible, but the code and comment must line up before
   merge.

6. **`ConsecutiveLeaderFailovers` reset semantics.** The field is only
   reset inside the good-epoch branch of Scenario 1. If Leader A fails
   once, Leader B runs normally, Leader A fails again, A.counter
   increments across non-adjacent epochs. Is "consecutive" per-Worker
   across all epochs (the written behavior), or per-Worker within a
   rolling window, or only across immediately adjacent elections? The
   implementation must pick one and document it.

### Attack surface

7. **P2P-network-partition DoS of the idle penalty.** An adversary who
   can eclipse a target Leader from the user request stream (P2P
   partition, BGP-level interception) forces Scenario 1 to fire every
   epoch — the Leader sees zero dispatches through no fault of its own.
   The document's "no deduction if entire model had no requests"
   carve-out is listed as optional and does not help (the attacker does
   not need to silence the whole model, only the target). Mitigation:
   make the carve-out mandatory, and add a second condition —
   only deduct when at least one other Leader on the same model *did*
   dispatch in the same epoch. The penalty then requires attacker to
   partition the model, not an individual Leader.

8. **Idle penalty is much heavier than failover penalty in practice.**
   Per-occurrence the numbers are -0.1 (idle) vs -0.05 (failover), but
   failover penalty only fires every 3 strikes. Cumulative: 10 idle
   epochs = -1.0; 10 failover events = -0.15. Idle is 6.7× heavier —
   but idle is also the penalty that is easiest to trigger from outside
   (see item 7). Recommend dropping idle to -0.03 (match single
   failover) or making the threshold gradual (partial penalty under
   partial-dispatch regimes).

9. **Recovery is slow relative to attack rate.** Decay +0.005/hour
   means recovering from 0.5 → 1.0 takes ~100 hours (4 days). A brief
   real outage can lock a legitimate Leader out for days. Either raise
   the decay step, or add a "N consecutive good epochs → extra +0.05"
   accelerator.

### Interaction with existing merged work

10. **S4 (PR #8) Leader pubkey distribution transient failure.** S4
    Workers accept AssignTask signed by the top-3 VRF Leaders for the
    current model. If a Leader's `LeaderReputationScore` drops sharply
    (e.g. one Scenario 1 hit from 1.0 to 0.9), it can fall out of the
    top-3 window mid-task. Result: Worker rejects the signature, a
    failover fires, and `ConsecutiveLeaderFailovers` increments —
    compounding the original penalty with a failover penalty for what
    was in fact a correct dispatch. Recommended mitigation: the
    top-3 window on the Worker side should be a union over the last
    few epochs (say the last 6 = 3 minutes), not strictly the current
    epoch. This smooths rep-driven transitions.

### Estimation caveat

11. **Implementation effort is undercounted.** The document's 200–300
    line figure omits the Proposer-side changes needed for Scenario 1
    (`SettlementEntry.LeaderAddress` writing), the test matrix (all
    seven phases need unit tests), and the VRF-formula branch logic.
    A realistic range is **550–700 lines** end-to-end. The P2
    classification does not change.

---

*These notes do not block this ingest. They are the hand-off brief for
whoever opens the implementation PR.*
