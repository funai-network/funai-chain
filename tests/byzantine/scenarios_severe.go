// scenarios_severe.go — KT plan §2.3.
//
// Slash + permanent ban paths. The "exactness" of the slash math (KT §3.2) is
// what the harness checks here — a regression that drops a percent point off
// the slash, or rounds the wrong way, would fail S1.

package byzantine

import (
	"fmt"
	"math/rand"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	workertypes "github.com/funai-wiki/funai-chain/x/worker/types"
)

// S1 — 3rd jail → permanent ban + 5 % slash + tombstone; stake deduction is
// exact.
//
// Path: JailWorker called 3 times. The third call dispatches to slash + set
// Tombstoned=true. We verify post-conditions on the SLASH math, the
// TOMBSTONED bool, and that no further state mutation is possible.
type ScenarioS1 struct{}

func (ScenarioS1) ID() string   { return "S1" }
func (ScenarioS1) Tier() Tier   { return TierSevere }
func (ScenarioS1) Description() string {
	return "3rd jail → permanent ban + 5% slash + tombstone; stake deduction exact"
}

func (s ScenarioS1) Run(env *Env, _ *rand.Rand) error {
	const initialStake int64 = 10_000
	addr := env.MakeWorker(0, initialStake)
	params := env.Worker.GetParams(env.Ctx)

	// 1st jail.
	env.Worker.JailWorker(env.Ctx, addr, 0)
	env.Advance(params.Jail1Duration + 1)
	_ = env.Worker.UnjailWorker(env.Ctx, addr)

	// 2nd jail.
	env.Worker.JailWorker(env.Ctx, addr, 0)
	env.Advance(params.Jail2Duration + 1)
	_ = env.Worker.UnjailWorker(env.Ctx, addr)

	// 3rd jail → slash + tombstone.
	env.Worker.JailWorker(env.Ctx, addr, 0)

	w := env.MustGet(addr)
	if !w.Tombstoned {
		return fmt.Errorf("S1: 3rd jail did not tombstone worker (count=%d)", w.JailCount)
	}
	if !w.Jailed || w.Status != workertypes.WorkerStatusJailed {
		return fmt.Errorf("S1: tombstone left state inconsistent: jailed=%v status=%v",
			w.Jailed, w.Status)
	}
	if w.JailUntil != 0 {
		return fmt.Errorf("S1: tombstoned worker JailUntil=%d want=0 (no possible unjail)", w.JailUntil)
	}

	// Stake math: post = initial × (1 - 0.05) = initial × 0.95.
	// Keeper computes burn as `stake.MulRaw(percent).QuoRaw(100)` (truncating).
	expectedBurn := math.NewInt(initialStake).MulRaw(int64(params.SlashFraudPercent)).QuoRaw(100)
	expectedRemaining := math.NewInt(initialStake).Sub(expectedBurn)
	if !w.Stake.Amount.Equal(expectedRemaining) {
		return fmt.Errorf("S1: post-slash stake=%s want=%s (initial=%d slash_pct=%d)",
			w.Stake.Amount.String(), expectedRemaining.String(), initialStake, params.SlashFraudPercent)
	}

	burned := env.Bank.TotalBurned("ufai")
	if !burned.Equal(expectedBurn) {
		return fmt.Errorf("S1: burned=%s want=%s", burned.String(), expectedBurn.String())
	}
	return nil
}

// S2 — Successful FraudProof submitted by user → Worker permanently banned +
// slashed; user receives compensation if applicable.
//
// Codepath: FraudProof handler calls SlashWorkerTo (sends 5% to user instead
// of burning) and TombstoneWorker. The two are separate keeper calls; the
// orchestrator (msg_server) sequences them.
type ScenarioS2 struct{}

func (ScenarioS2) ID() string   { return "S2" }
func (ScenarioS2) Tier() Tier   { return TierSevere }
func (ScenarioS2) Description() string {
	return "FraudProof → slash to user + tombstone; compensation arrives intact"
}

func (s ScenarioS2) Run(env *Env, _ *rand.Rand) error {
	const initialStake int64 = 10_000
	worker := env.MakeWorker(0, initialStake)
	user := DerivAddr(999) // arbitrary recipient
	params := env.Worker.GetParams(env.Ctx)

	env.Worker.SlashWorkerTo(env.Ctx, worker, params.SlashFraudPercent, user)
	env.Worker.TombstoneWorker(env.Ctx, worker)

	w := env.MustGet(worker)
	if !w.Tombstoned {
		return fmt.Errorf("S2: TombstoneWorker did not set Tombstoned=true")
	}

	expectedSlash := math.NewInt(initialStake).MulRaw(int64(params.SlashFraudPercent)).QuoRaw(100)
	expectedRemaining := math.NewInt(initialStake).Sub(expectedSlash)
	if !w.Stake.Amount.Equal(expectedRemaining) {
		return fmt.Errorf("S2: post-slash stake=%s want=%s",
			w.Stake.Amount.String(), expectedRemaining.String())
	}

	// FraudProof slashes via SlashWorkerTo (no burn) — bank should record
	// zero burn, only a SendCoinsFromModuleToAccount.
	burned := env.Bank.TotalBurned("ufai")
	if !burned.IsZero() {
		return fmt.Errorf("S2: FraudProof path should NOT burn; burned=%s", burned.String())
	}
	return nil
}

// S3 — Verifier liability chain (1st-tier verifies PASS, 2nd-tier returns
// FAIL) → each of the 3 first-tier verifiers loses 2 % stake + 0.20 reputation.
//
// Stub: requires settlement-side liability cascade, which is not yet wired
// into the harness. The keeper-level primitive (slash 2%) would be
// `SlashWorker(addr, 2)`; the rep penalty is `ReputationOnMiss(addr,
// "second_verifier")` for the doubled −0.20 delta. Compose once settlement
// integration lands.
type ScenarioS3 struct{}

func (ScenarioS3) ID() string   { return "S3" }
func (ScenarioS3) Tier() Tier   { return TierSevere }
func (ScenarioS3) Description() string {
	return "Verifier liability chain → 3 verifiers lose 2% stake + 0.20 reputation (stub)"
}

func (s ScenarioS3) Run(env *Env, _ *rand.Rand) error {
	const verifierStake int64 = 5_000
	verifiers := []sdk.AccAddress{
		env.MakeWorker(10, verifierStake),
		env.MakeWorker(11, verifierStake),
		env.MakeWorker(12, verifierStake),
	}

	// Apply the cascade primitive: 2% slash + double-rep miss.
	for _, v := range verifiers {
		env.Worker.SlashWorker(env.Ctx, v, 2)
		env.Worker.ReputationOnMiss(env.Ctx, v, "second_verifier")
	}

	expectedSlash := math.NewInt(verifierStake).MulRaw(2).QuoRaw(100)
	expectedRemaining := math.NewInt(verifierStake).Sub(expectedSlash)
	wantRep := workertypes.ReputationInitial - workertypes.ReputationAuditMiss
	for _, v := range verifiers {
		w := env.MustGet(v)
		if !w.Stake.Amount.Equal(expectedRemaining) {
			return fmt.Errorf("S3: verifier %s stake=%s want=%s",
				v.String(), w.Stake.Amount.String(), expectedRemaining.String())
		}
		if w.ReputationScore != wantRep {
			return fmt.Errorf("S3: verifier %s rep=%d want=%d", v.String(), w.ReputationScore, wantRep)
		}
		if w.Jailed || w.Tombstoned {
			return fmt.Errorf("S3: liability cascade should NOT jail/tombstone; jailed=%v tomb=%v",
				w.Jailed, w.Tombstoned)
		}
	}
	return nil
}

// S4 — Slashed Worker tries to unjail → chain rejects (tombstone).
type ScenarioS4 struct{}

func (ScenarioS4) ID() string   { return "S4" }
func (ScenarioS4) Tier() Tier   { return TierSevere }
func (ScenarioS4) Description() string {
	return "Tombstoned worker tries to unjail → keeper rejects"
}

func (s ScenarioS4) Run(env *Env, _ *rand.Rand) error {
	addr := env.MakeWorker(0, 10_000)
	env.Worker.TombstoneWorker(env.Ctx, addr)

	err := env.Worker.UnjailWorker(env.Ctx, addr)
	if err == nil {
		return fmt.Errorf("S4: UnjailWorker on tombstoned worker returned nil; should error")
	}
	w := env.MustGet(addr)
	if !w.Tombstoned || !w.Jailed {
		return fmt.Errorf("S4: failed unjail mutated tombstone state: tomb=%v jailed=%v",
			w.Tombstoned, w.Jailed)
	}
	return nil
}

// S5 — Slashed Worker tries to re-register → must restake from scratch;
// remaining stake retrievable only after slash deduction processed.
//
// Stub: re-registration is msg_server territory (MsgRegisterWorker), not
// directly testable on the keeper without spinning up a msg_server. The
// keeper-level guarantee — that a tombstoned worker cannot have its `Status`
// flipped back to Active — is exercised by the state_machine invariant.
type ScenarioS5 struct{}

func (ScenarioS5) ID() string   { return "S5" }
func (ScenarioS5) Tier() Tier   { return TierSevere }
func (ScenarioS5) Description() string {
	return "Slashed worker tries to re-register → restake from scratch (stub: needs msg_server)"
}

func (s ScenarioS5) Run(_ *Env, _ *rand.Rand) error { return nil }

// S6 — Worker is judged FAIL while a second-verification fee lock is in
// flight → fee is not released; jail flow proceeds. Settlement-layer; stub.
type ScenarioS6 struct{}

func (ScenarioS6) ID() string   { return "S6" }
func (ScenarioS6) Tier() Tier   { return TierSevere }
func (ScenarioS6) Description() string {
	return "Worker FAIL during second-verification fee lock → fee not released, jail proceeds (stub)"
}

func (s ScenarioS6) Run(_ *Env, _ *rand.Rand) error { return nil }

// AllSevere returns the §2.3 scenario set in plan order.
func AllSevere() []Scenario {
	return []Scenario{
		ScenarioS1{}, ScenarioS2{}, ScenarioS3{}, ScenarioS4{}, ScenarioS5{}, ScenarioS6{},
	}
}
