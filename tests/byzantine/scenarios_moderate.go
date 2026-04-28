// scenarios_moderate.go — KT plan §2.2.
//
// Jail-triggering paths. M7 and M8 are the load-bearing ones — they validate
// PR #28's "1000-task decay" rule (replaced V5.2's "50-task reset" per the
// KT V6 Byzantine plan, 2026-04-27). A regression in `IncrementSuccessStreak`
// would surface here first.

package byzantine

import (
	"fmt"
	"math/rand"

	workertypes "github.com/funai-wiki/funai-chain/x/worker/types"
)

// M1 — Sliding window: 3 misses across 10 tasks → 1st jail (120 blocks).
//
// The "3-in-10" detection is orchestrator logic (currently lives at the
// p2p/leader layer); the harness simulates that detection firing by calling
// JailWorker directly after the 3 misses are recorded. The keeper-side
// guarantee being tested is: GIVEN a JailWorker call, the resulting state is
// JailCount=1, Status=Jailed, JailUntil=now+Jail1Duration.
type ScenarioM1 struct{}

func (ScenarioM1) ID() string   { return "M1" }
func (ScenarioM1) Tier() Tier   { return TierModerate }
func (ScenarioM1) Description() string {
	return "3 misses across 10 tasks → 1st jail (120 blocks), reputation −0.30"
}

func (s ScenarioM1) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)

	// 3 misses (orchestrator's detection logic would have called these).
	for i := 0; i < 3; i++ {
		env.Worker.ReputationOnMiss(env.Ctx, addr, "worker")
	}
	env.Worker.JailWorker(env.Ctx, addr, 0)

	w := env.MustGet(addr)
	params := env.Worker.GetParams(env.Ctx)
	if w.JailCount != 1 {
		return fmt.Errorf("M1: JailCount=%d want=1", w.JailCount)
	}
	if !w.Jailed {
		return fmt.Errorf("M1: should be jailed")
	}
	if w.Status != workertypes.WorkerStatusJailed {
		return fmt.Errorf("M1: Status=%v want=Jailed", w.Status)
	}
	wantUntil := env.Height() + params.Jail1Duration
	if w.JailUntil != wantUntil {
		return fmt.Errorf("M1: JailUntil=%d want=%d", w.JailUntil, wantUntil)
	}
	wantRep := workertypes.ReputationInitial - 3*workertypes.ReputationMissDelta
	if w.ReputationScore != wantRep {
		return fmt.Errorf("M1: ReputationScore=%d want=%d", w.ReputationScore, wantRep)
	}
	return nil
}

// M2 — Worker fails to submit batch log → every task in the batch is FAIL,
// 1st jail. From the Worker keeper's perspective, equivalent to M1 (jail
// trigger fires); per-task FAIL settlement is settlement-layer.
type ScenarioM2 struct{}

func (ScenarioM2) ID() string   { return "M2" }
func (ScenarioM2) Tier() Tier   { return TierModerate }
func (ScenarioM2) Description() string {
	return "Worker fails to submit batch log → tasks FAIL, 1st jail (Worker-side state)"
}

func (s ScenarioM2) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	env.Worker.JailWorker(env.Ctx, addr, 0)

	w := env.MustGet(addr)
	if w.JailCount != 1 || !w.Jailed {
		return fmt.Errorf("M2: jail did not fire (jail_count=%d jailed=%v)", w.JailCount, w.Jailed)
	}
	return nil
}

// M3 — Verification FAIL (Worker logits ≠ Verifier logits) → task FAIL, fee
// not settled, 1st jail. Worker-side state same as M1/M2; the differentiating
// detail (fee not settled) is settlement-layer.
type ScenarioM3 struct{}

func (ScenarioM3) ID() string   { return "M3" }
func (ScenarioM3) Tier() Tier   { return TierModerate }
func (ScenarioM3) Description() string {
	return "Verification FAIL → task FAIL, 1st jail (Worker-side state)"
}

func (s ScenarioM3) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	env.Worker.JailWorker(env.Ctx, addr, 0)
	w := env.MustGet(addr)
	if w.JailCount != 1 || !w.Jailed {
		return fmt.Errorf("M3: jail did not fire")
	}
	return nil
}

// M4 — Worker timeouts exceed sliding-window threshold → jail; all unfinished
// tasks redispatched. Worker-side: same as M1. Redispatch is p2p-layer.
type ScenarioM4 struct{}

func (ScenarioM4) ID() string   { return "M4" }
func (ScenarioM4) Tier() Tier   { return TierModerate }
func (ScenarioM4) Description() string {
	return "Worker timeouts exceed threshold → jail (Worker-side state)"
}

func (s ScenarioM4) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	env.Worker.JailWorker(env.Ctx, addr, 0)
	w := env.MustGet(addr)
	if w.JailCount != 1 {
		return fmt.Errorf("M4: JailCount=%d want=1", w.JailCount)
	}
	return nil
}

// M5 — After 1st jail → unjail → reoffend → 2nd jail (720 blocks).
//
// Tests the progressive-jail-duration rule directly. UnjailWorker leaves
// JailCount intact, so the next JailWorker call sees JailCount=1 and
// applies Jail2Duration.
type ScenarioM5 struct{}

func (ScenarioM5) ID() string   { return "M5" }
func (ScenarioM5) Tier() Tier   { return TierModerate }
func (ScenarioM5) Description() string {
	return "1st jail → unjail → reoffend → 2nd jail (720 blocks)"
}

func (s ScenarioM5) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	params := env.Worker.GetParams(env.Ctx)

	env.Worker.JailWorker(env.Ctx, addr, 0)
	if w := env.MustGet(addr); w.JailCount != 1 {
		return fmt.Errorf("M5: after 1st jail JailCount=%d want=1", w.JailCount)
	}

	// Advance past Jail1Duration so unjail succeeds.
	env.Advance(params.Jail1Duration + 1)
	if err := env.Worker.UnjailWorker(env.Ctx, addr); err != nil {
		return fmt.Errorf("M5: unjail failed: %v", err)
	}
	if w := env.MustGet(addr); w.Jailed || w.JailCount != 1 {
		return fmt.Errorf("M5: after unjail jailed=%v jail_count=%d (want jailed=false count=1)",
			w.Jailed, w.JailCount)
	}

	// Reoffend.
	env.Worker.JailWorker(env.Ctx, addr, 0)
	w := env.MustGet(addr)
	if w.JailCount != 2 {
		return fmt.Errorf("M5: after 2nd jail JailCount=%d want=2", w.JailCount)
	}
	wantUntil := env.Height() + params.Jail2Duration
	if w.JailUntil != wantUntil {
		return fmt.Errorf("M5: JailUntil=%d want=%d (Jail2Duration=%d)",
			w.JailUntil, wantUntil, params.Jail2Duration)
	}
	return nil
}

// M6 — Worker tries to accept tasks while jailed → chain rejects; in_flight
// does not grow. Stubbed: requires accept-task wiring on the keeper. Wire
// when the dispatch capacity scenarios land.
type ScenarioM6 struct{}

func (ScenarioM6) ID() string   { return "M6" }
func (ScenarioM6) Tier() Tier   { return TierModerate }
func (ScenarioM6) Description() string {
	return "Worker tries to accept tasks while jailed → rejected (stub: needs dispatch wiring)"
}

func (s ScenarioM6) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	env.Worker.JailWorker(env.Ctx, addr, 0)
	w := env.MustGet(addr)
	// Sanity: a jailed Worker reports IsActive() == false, which is the
	// boolean the dispatch path checks before attempting an accept.
	if w.IsActive() {
		return fmt.Errorf("M6: jailed worker reports IsActive=true (dispatch path would still hand it tasks)")
	}
	return nil
}

// M7 — After 1st jail, completes 999 honest tasks (decay threshold not
// reached), then reoffends. JailCount still 1, reoffence goes straight to
// 2nd jail (720 blocks).
//
// **Load-bearing**: validates PR #28 (commit 823f642). KT V6 Byzantine plan
// 2026-04-27 specifically called out the "rhythm-cheating at constant cost"
// failure mode — a worker who games the V5.2 50-task reset by misbehaving
// just under threshold. The 1000-task decay is what closes that.
type ScenarioM7 struct{}

func (ScenarioM7) ID() string   { return "M7" }
func (ScenarioM7) Tier() Tier   { return TierModerate }
func (ScenarioM7) Description() string {
	return "After 1st jail, 999 honest tasks → JailCount stays 1 → reoffend = 2nd jail"
}

func (s ScenarioM7) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	params := env.Worker.GetParams(env.Ctx)

	env.Worker.JailWorker(env.Ctx, addr, 0)
	env.Advance(params.Jail1Duration + 1)
	_ = env.Worker.UnjailWorker(env.Ctx, addr)

	// 999 honest successes — one short of the decay step.
	for i := uint32(0); i < params.JailDecayInterval-1; i++ {
		env.Worker.IncrementSuccessStreak(env.Ctx, addr)
	}
	w := env.MustGet(addr)
	if w.JailCount != 1 {
		return fmt.Errorf("M7: after 999 successes JailCount=%d want=1 (decay should NOT have fired)", w.JailCount)
	}
	if w.SuccessStreak != params.JailDecayInterval-1 {
		return fmt.Errorf("M7: after 999 successes SuccessStreak=%d want=%d",
			w.SuccessStreak, params.JailDecayInterval-1)
	}

	// Reoffend → JailCount goes to 2 → Jail2Duration.
	env.Worker.JailWorker(env.Ctx, addr, 0)
	w = env.MustGet(addr)
	if w.JailCount != 2 {
		return fmt.Errorf("M7: reoffend JailCount=%d want=2", w.JailCount)
	}
	if w.JailUntil != env.Height()+params.Jail2Duration {
		return fmt.Errorf("M7: JailUntil mismatch — should use Jail2Duration since count was still 1 at reoffend")
	}
	return nil
}

// M8 — After 1st jail, completes exactly 1000 honest tasks (one decay step).
// JailCount decays to 0; reoffence goes back to 1st jail.
//
// **Load-bearing**: complementary to M7. Together they pin both sides of the
// JailDecayInterval boundary.
type ScenarioM8 struct{}

func (ScenarioM8) ID() string   { return "M8" }
func (ScenarioM8) Tier() Tier   { return TierModerate }
func (ScenarioM8) Description() string {
	return "After 1st jail, 1000 honest tasks → JailCount decays to 0 → reoffend = 1st jail"
}

func (s ScenarioM8) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	params := env.Worker.GetParams(env.Ctx)

	env.Worker.JailWorker(env.Ctx, addr, 0)
	env.Advance(params.Jail1Duration + 1)
	_ = env.Worker.UnjailWorker(env.Ctx, addr)

	// Exactly 1000 honest successes.
	for i := uint32(0); i < params.JailDecayInterval; i++ {
		env.Worker.IncrementSuccessStreak(env.Ctx, addr)
	}
	w := env.MustGet(addr)
	if w.JailCount != 0 {
		return fmt.Errorf("M8: after %d successes JailCount=%d want=0", params.JailDecayInterval, w.JailCount)
	}
	if w.SuccessStreak != 0 {
		return fmt.Errorf("M8: after decay step SuccessStreak=%d want=0 (should reset)", w.SuccessStreak)
	}

	// Reoffend → JailCount goes to 1 → Jail1Duration.
	env.Worker.JailWorker(env.Ctx, addr, 0)
	w = env.MustGet(addr)
	if w.JailCount != 1 {
		return fmt.Errorf("M8: reoffend JailCount=%d want=1 (decay should have reset back to 1st-jail behaviour)",
			w.JailCount)
	}
	if w.JailUntil != env.Height()+params.Jail1Duration {
		return fmt.Errorf("M8: JailUntil should use Jail1Duration after decay")
	}
	return nil
}

// AllModerate returns the §2.2 scenario set in plan order.
func AllModerate() []Scenario {
	return []Scenario{
		ScenarioM1{}, ScenarioM2{}, ScenarioM3{}, ScenarioM4{},
		ScenarioM5{}, ScenarioM6{}, ScenarioM7{}, ScenarioM8{},
	}
}
