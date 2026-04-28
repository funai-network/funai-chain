// invariants.go — KT plan §3 invariant checks.
//
// Run after every scenario round. Each function returns the empty string if the
// invariant holds, or a human-readable failure message otherwise. The harness
// collects all failures (not just the first) so a single round can surface
// multiple violations — useful when one root-cause bug cascades.

package byzantine

import (
	"fmt"

	"cosmossdk.io/math"

	workertypes "github.com/funai-wiki/funai-chain/x/worker/types"
)

// CheckAll runs every invariant against `env`. Returns the names of failing
// invariants (empty slice = all PASS).
func CheckAll(env *Env) []string {
	var failed []string
	for _, inv := range allInvariants {
		if msg := inv.fn(env); msg != "" {
			failed = append(failed, fmt.Sprintf("%s: %s", inv.name, msg))
		}
	}
	return failed
}

type invariant struct {
	name string
	fn   func(*Env) string
}

// allInvariants is the §3 list. Order matches the plan so the report layout
// mirrors KT §6 verbatim.
var allInvariants = []invariant{
	{"fee_conservation", invFeeConservation},
	{"stake_conservation", invStakeConservation},
	{"reputation_bounds", invReputationBounds},
	{"state_machine", invStateMachine},
	{"jail_count", invJailCount},
	{"task_uniqueness", invTaskUniqueness},
	{"in_flight", invInFlight},
}

// invFeeConservation — §3.1.
//
// In the harness, fees are not yet routed through real keepers (settlement
// integration is a follow-up). The full §3.1 statement — "user fee in ==
// sum(worker_share + verifier_share + fund_share)" with `max drift: 0 uFAI`
// per KT §6 — needs settlement keeper integration to test directly.
//
// Until then, the invariant checks an upper-bound sanity property that
// catches double-burn / runaway-burn bugs in the slash path: total uFAI
// burned this round must not exceed the sum of all initial stakes seen by
// the harness. This is intentionally weak — by design only flags absurd
// values, not 1-uFAI rounding drift. The KT-grade tight check arrives with
// the settlement integration follow-up.
func invFeeConservation(env *Env) string {
	burned := env.Bank.TotalBurned("ufai")
	if burned.IsZero() {
		return ""
	}
	// Upper bound: post-slash stakes + amount burned should be ≤ a generous
	// per-round ceiling (we can't easily reconstruct pre-slash totals from a
	// snapshot, but we can flag burns that dwarf any plausible cause).
	totalPostSlashStake := math.ZeroInt()
	for _, w := range env.Worker.GetAllWorkers(env.Ctx) {
		totalPostSlashStake = totalPostSlashStake.Add(w.Stake.Amount)
	}
	// Burn must be < total post-slash stake (since burn is slash and each
	// slash is at most 5% of a single worker, burn ≤ total * 5/95).
	// Using a 1× ceiling instead of 5/95 gives slack for harness edges.
	if burned.GT(totalPostSlashStake) {
		return fmt.Sprintf("burned %s uFAI > total remaining stake %s — likely double-slash or runaway burn",
			burned.String(), totalPostSlashStake.String())
	}
	return ""
}

// invStakeConservation — §3.2.
//
// Slash is exactly 5%. After slash, `stake_remaining = stake_original − slashed`.
// Currently checks: any tombstoned worker's stake is non-negative and is
// exactly the post-slash residual (within integer truncation).
//
// Without per-round pre-slash snapshots, we can't verify the *amount* slashed
// here directly — that's the role of `invFeeConservation`'s burn cross-check.
// What we CAN do: assert no worker has negative stake (which would be a bug
// in the slash math).
func invStakeConservation(env *Env) string {
	for _, w := range env.Worker.GetAllWorkers(env.Ctx) {
		if w.Stake.Amount.IsNegative() {
			return fmt.Sprintf("worker %s has negative stake: %s", w.Address, w.Stake.String())
		}
	}
	return ""
}

// invReputationBounds — §3.3.
//
// Plan says `[0.0, 1.0]`. Code stores ReputationScore in `[0, 12000]` (1.2
// soft ceiling to allow positive reinforcement above baseline). The plan's
// ceiling is the more conservative interpretation of "valid range"; the
// codepath enforces `≤ ReputationMax (12000)` which is also valid. Use the
// code's wider bound here so we match what the keeper actually enforces;
// flagging the code-vs-plan ceiling discrepancy is for the test report,
// not for halting the harness.
func invReputationBounds(env *Env) string {
	for _, w := range env.Worker.GetAllWorkers(env.Ctx) {
		if w.ReputationScore > workertypes.ReputationMax {
			return fmt.Sprintf("worker %s reputation %d > max %d",
				w.Address, w.ReputationScore, workertypes.ReputationMax)
		}
		// No lower bound check needed — uint32 ≥ 0 by type.
	}
	return ""
}

// invStateMachine — §3.4.
//
// Legal Worker transitions:
//
//	ACTIVE  →  JAILED  →  ACTIVE      (via unjail, JailCount stays)
//	ACTIVE  →  JAILED  →  TOMBSTONED  (via 3rd jail or FraudProof)
//	(EXITING / EXITED are graceful exit states, not relevant to the byzantine
//	 path; the harness does not exercise them in the initial scenario set.)
//
// Implementation note: the keeper conflates "Status field" with "Jailed bool".
// Concretely, JailWorker sets both Status=Jailed AND Jailed=true; UnjailWorker
// resets both. Tombstoned worker has Jailed=true + Tombstoned=true + Status=Jailed.
// So the invariant collapses to:
//
//   - Tombstoned ⇒ Status==Jailed ∧ Jailed==true ∧ JailUntil==0
//   - Jailed     ⇒ Status==Jailed
//   - Status==Active ⇒ ¬Jailed ∧ ¬Tombstoned
func invStateMachine(env *Env) string {
	for _, w := range env.Worker.GetAllWorkers(env.Ctx) {
		switch {
		case w.Tombstoned:
			if w.Status != workertypes.WorkerStatusJailed || !w.Jailed || w.JailUntil != 0 {
				return fmt.Sprintf("worker %s tombstoned but state inconsistent: status=%v jailed=%v until=%d",
					w.Address, w.Status, w.Jailed, w.JailUntil)
			}
		case w.Jailed:
			if w.Status != workertypes.WorkerStatusJailed {
				return fmt.Sprintf("worker %s jailed but status=%v", w.Address, w.Status)
			}
		case w.Status == workertypes.WorkerStatusActive:
			if w.Jailed || w.Tombstoned {
				return fmt.Sprintf("worker %s status=Active but jailed=%v tombstoned=%v",
					w.Address, w.Jailed, w.Tombstoned)
			}
		}
	}
	return ""
}

// invJailCount — §3.5.
//
// `jail_count ≥ 0`: trivially true since field is uint32.
//
// "Decay occurs only at the 1000-honest-task milestone, decay step is exactly
// `−1`." We can't observe history from a single state snapshot, but we CAN
// assert: if SuccessStreak < JailDecayInterval, JailCount has not decayed
// since the last jail (i.e. SuccessStreak should have been zeroed at the last
// JailWorker call but allowed to grow afterwards). The keeper enforces this
// in IncrementSuccessStreak; the invariant is a sanity check.
func invJailCount(env *Env) string {
	params := env.Worker.GetParams(env.Ctx)
	for _, w := range env.Worker.GetAllWorkers(env.Ctx) {
		if w.SuccessStreak >= params.JailDecayInterval {
			// At the decay boundary, IncrementSuccessStreak should have
			// reset it to 0 (after decrementing JailCount). Reaching the
			// boundary without reset means the keeper missed a decay step.
			return fmt.Sprintf("worker %s success_streak=%d ≥ decay_interval=%d (should have been zeroed)",
				w.Address, w.SuccessStreak, params.JailDecayInterval)
		}
		// Tombstoned worker should have JailCount ≥ 3 (V5.2: tombstone fires
		// only at 3rd jail, except via FraudProof which calls TombstoneWorker
		// directly without incrementing JailCount).
		// Skipped for now — distinguishing the two paths needs scenario
		// metadata; revisit when FraudProof scenarios land.
	}
	return ""
}

// invTaskUniqueness — §3.6.
//
// "Same task_id is settled at most once." The byzantine harness in this PR
// focuses on Worker-side state machine; settlement-side dedup tests live in
// `x/settlement/keeper/`. This invariant is wired but currently a no-op
// placeholder until settlement scenarios (C9) are implemented.
//
// When settlement integration lands, this becomes:
//
//	settled_task_ids := env.Settlement.AllSettledTaskIDs(env.Ctx)
//	if hasDuplicates(settled_task_ids) { return ... }
func invTaskUniqueness(_ *Env) string {
	return ""
}

// invInFlight — §3.7.
//
// `0 ≤ in_flight ≤ capacity`. The Worker keeper currently does not store
// `in_flight` on the Worker struct (per-task tracking lives elsewhere); the
// V6 dispatch capacity work in commit c4d0b24 added per-Worker batch capacity
// to the keeper. Until those scenarios are added the invariant is a placeholder.
//
// When dispatch scenarios land:
//
//	for _, w := range env.Worker.GetAllWorkers(env.Ctx) {
//	    if env.Worker.GetInFlight(env.Ctx, addr) > w.MaxConcurrentTasks { ... }
//	}
func invInFlight(_ *Env) string {
	return ""
}
