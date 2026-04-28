// scenarios_light.go — KT plan §2.1.
//
// Reputation-only paths. No jail fires. These scenarios exist mainly to detect
// regressions in the reputation accounting (e.g. miss penalty changes from 0.10
// to 0.05 silently); they have minimal coupling to the harness, which makes
// them good smoke tests for the framework itself.

package byzantine

import (
	"fmt"
	"math/rand"

	workertypes "github.com/funai-wiki/funai-chain/x/worker/types"
)

// L1 — Worker misses 1 task occasionally. Reputation −0.10, no jail, task
// would be redispatched (redispatch tested at p2p layer; out of scope here).
type ScenarioL1 struct{}

func (ScenarioL1) ID() string   { return "L1" }
func (ScenarioL1) Tier() Tier   { return TierLight }
func (ScenarioL1) Description() string {
	return "Worker misses 1 task occasionally → reputation −0.10, no jail"
}

func (s ScenarioL1) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)

	env.Worker.ReputationOnMiss(env.Ctx, addr, "worker")

	w := env.MustGet(addr)
	want := workertypes.ReputationInitial - workertypes.ReputationMissDelta
	if w.ReputationScore != want {
		return fmt.Errorf("L1: ReputationScore=%d want=%d", w.ReputationScore, want)
	}
	if w.Jailed || w.JailCount != 0 {
		return fmt.Errorf("L1: should not be jailed; jailed=%v jail_count=%d", w.Jailed, w.JailCount)
	}
	return nil
}

// L2 — Worker misses 2 consecutive tasks (still under threshold). Reputation
// −0.20, no jail.
type ScenarioL2 struct{}

func (ScenarioL2) ID() string   { return "L2" }
func (ScenarioL2) Tier() Tier   { return TierLight }
func (ScenarioL2) Description() string {
	return "Worker misses 2 consecutive tasks → reputation −0.20, no jail"
}

func (s ScenarioL2) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)

	env.Worker.ReputationOnMiss(env.Ctx, addr, "worker")
	env.Worker.ReputationOnMiss(env.Ctx, addr, "worker")

	w := env.MustGet(addr)
	want := workertypes.ReputationInitial - 2*workertypes.ReputationMissDelta
	if w.ReputationScore != want {
		return fmt.Errorf("L2: ReputationScore=%d want=%d", w.ReputationScore, want)
	}
	if w.Jailed || w.JailCount != 0 {
		return fmt.Errorf("L2: should not be jailed after 2 misses (orchestrator decides; keeper doesn't auto-jail on miss)")
	}
	return nil
}

// L3 — Worker voluntarily lowers `capacity`. On-chain capacity updated;
// in_flight unaffected.
//
// Stub: requires an "update capacity" path on the keeper that does not yet
// exist as a single-call API (capacity changes today flow through Worker
// re-registration via msg_server). Wire when that path lands.
type ScenarioL3 struct{}

func (ScenarioL3) ID() string   { return "L3" }
func (ScenarioL3) Tier() Tier   { return TierLight }
func (ScenarioL3) Description() string {
	return "Worker voluntarily lowers capacity → capacity updated, in_flight unchanged (stub)"
}

func (s ScenarioL3) Run(env *Env, rng *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	// Seed an arbitrary starting capacity in [4, 16] so "lower" has a range
	// to choose from (default capacity 0 / 1 has nothing strictly lower).
	startCap := uint32(4 + rng.Intn(13))
	w := env.MustGet(addr)
	w.MaxConcurrentTasks = startCap
	env.Worker.SetWorker(env.Ctx, w)

	// Lower to a value strictly less than start. Direct field write —
	// simulates msg_server.UpdateCapacity once that lands.
	newCap := uint32(1 + rng.Intn(int(startCap-1)))
	w.MaxConcurrentTasks = newCap
	env.Worker.SetWorker(env.Ctx, w)

	got := env.MustGet(addr)
	if got.MaxConcurrentTasks >= startCap {
		return fmt.Errorf("L3: capacity did not lower: %d → %d", startCap, got.MaxConcurrentTasks)
	}
	if got.MaxConcurrentTasks != newCap {
		return fmt.Errorf("L3: SetWorker did not persist new capacity: got=%d want=%d",
			got.MaxConcurrentTasks, newCap)
	}
	return nil
}

// L4 — Worker is slow but does not time out. Settles normally, no penalty.
//
// Encoded as: complete a successful task chain (ReputationOnAccept) and verify
// no rep loss occurred. The "timeout vs in-window" decision is made at the
// orchestrator (p2p layer) which calls ReputationOnAccept on success and
// ReputationOnMiss on timeout — the harness exercises only the former here.
type ScenarioL4 struct{}

func (ScenarioL4) ID() string   { return "L4" }
func (ScenarioL4) Tier() Tier   { return TierLight }
func (ScenarioL4) Description() string {
	return "Worker is slow but does not time out → settles normally, no penalty"
}

func (s ScenarioL4) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	pre := env.MustGet(addr).ReputationScore

	env.Worker.ReputationOnAccept(env.Ctx, addr)

	w := env.MustGet(addr)
	if w.Jailed || w.JailCount != 0 {
		return fmt.Errorf("L4: should not be jailed after on-time success")
	}
	want := pre + workertypes.ReputationAcceptDelta
	if want > workertypes.ReputationMax {
		want = workertypes.ReputationMax
	}
	if w.ReputationScore != want {
		return fmt.Errorf("L4: ReputationScore=%d want=%d (started %d)", w.ReputationScore, want, pre)
	}
	return nil
}

// L5 — Verifier is slow but submits inside the verification window. Receives
// the verifier fee normally.
//
// Settlement-side fee accounting is not yet wired into the harness. Stubbed as
// a no-op until settlement keeper integration lands.
type ScenarioL5 struct{}

func (ScenarioL5) ID() string   { return "L5" }
func (ScenarioL5) Tier() Tier   { return TierLight }
func (ScenarioL5) Description() string {
	return "Verifier is slow but submits inside window → receives fee (stub: needs settlement integration)"
}

func (s ScenarioL5) Run(_ *Env, _ *rand.Rand) error {
	// Stub. Real check requires fee routing through settlement keeper.
	return nil
}

// AllLight returns the §2.1 scenario set in plan order.
func AllLight() []Scenario {
	return []Scenario{
		ScenarioL1{}, ScenarioL2{}, ScenarioL3{}, ScenarioL4{}, ScenarioL5{},
	}
}
