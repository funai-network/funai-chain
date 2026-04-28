// Package byzantine implements the V6 penalty-path stress harness.
//
// Each scenario in `docs/testing/FunAI_V6_Byzantine_Test_Plan_KT.md` is one
// `Scenario` here; each is run for N parameter-randomised rounds (default 100
// per PR, 10 000 nightly via `-tags byzantine_full`). After every round the
// harness re-checks the seven invariants from the plan; any single violation
// halts the run and prints the seed for repro.
//
// No real GPU or network. State machine only — exactly what KT §1 calls for.
package byzantine

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	workerkeeper "github.com/funai-wiki/funai-chain/x/worker/keeper"
	workertypes "github.com/funai-wiki/funai-chain/x/worker/types"
)

// Tier groups scenarios for the §6 report layout.
type Tier string

const (
	TierLight    Tier = "light"
	TierModerate Tier = "moderate"
	TierSevere   Tier = "severe"
	TierCombined Tier = "combined"
)

// Scenario is one row in the KT plan §2 tables.
//
// `Run` receives a fresh environment plus a per-round PRNG seeded from the
// round index — this gives reproducible failures: if scenario M5 round 743
// fails, re-run with the same seed and the bug is back.
type Scenario interface {
	ID() string
	Tier() Tier
	Description() string
	Run(env *Env, rng *rand.Rand) error
}

// Env wraps a fresh in-memory chain context for one round. Every round gets a
// new Env so cross-round state pollution is impossible — a scenario that mutates
// 50 workers and then exits cannot bleed into the next round.
type Env struct {
	Ctx      sdk.Context
	Worker   workerkeeper.Keeper
	Bank     *MockBank
	height   int64
	maxBlock int64
}

// MockBank tracks token movements so the Fee/Stake conservation invariants can
// inspect post-condition balances without going through a real bank module.
//
// Records every send/burn as a delta keyed by `(module|account, denom)`. The
// scenarios call Worker keeper methods which in turn call these — the
// post-condition invariant just sums the deltas.
type MockBank struct {
	// burned[denom] = total uFAI burned
	burned map[string]math.Int
	// moved[from->to] = total uFAI moved (signed: positive = out of from, in to)
	moved map[string]map[string]math.Int
}

func NewMockBank() *MockBank {
	return &MockBank{
		burned: map[string]math.Int{},
		moved:  map[string]map[string]math.Int{},
	}
}

func (m *MockBank) SendCoins(_ context.Context, from, to sdk.AccAddress, amt sdk.Coins) error {
	m.recordMove(from.String(), to.String(), amt)
	return nil
}

func (m *MockBank) SendCoinsFromAccountToModule(_ context.Context, from sdk.AccAddress, mod string, amt sdk.Coins) error {
	m.recordMove(from.String(), "module:"+mod, amt)
	return nil
}

func (m *MockBank) SendCoinsFromModuleToAccount(_ context.Context, mod string, to sdk.AccAddress, amt sdk.Coins) error {
	m.recordMove("module:"+mod, to.String(), amt)
	return nil
}

func (m *MockBank) BurnCoins(_ context.Context, mod string, amt sdk.Coins) error {
	for _, c := range amt {
		cur, ok := m.burned[c.Denom]
		if !ok {
			cur = math.ZeroInt()
		}
		m.burned[c.Denom] = cur.Add(c.Amount)
	}
	_ = mod
	return nil
}

func (m *MockBank) recordMove(from, to string, amt sdk.Coins) {
	if _, ok := m.moved[from]; !ok {
		m.moved[from] = map[string]math.Int{}
	}
	for _, c := range amt {
		cur, ok := m.moved[from][c.Denom]
		if !ok {
			cur = math.ZeroInt()
		}
		m.moved[from][c.Denom] = cur.Add(c.Amount)
	}
	if _, ok := m.moved[to]; !ok {
		m.moved[to] = map[string]math.Int{}
	}
	for _, c := range amt {
		cur, ok := m.moved[to][c.Denom]
		if !ok {
			cur = math.ZeroInt()
		}
		m.moved[to][c.Denom] = cur.Add(c.Amount)
	}
}

// TotalBurned is the cumulative burn for one denom across this round.
func (m *MockBank) TotalBurned(denom string) math.Int {
	if v, ok := m.burned[denom]; ok {
		return v
	}
	return math.ZeroInt()
}

// NewEnv stands up a fresh in-memory worker keeper with default params.
//
// Block height starts at 100 (matches existing keeper_test.go convention). The
// scenario can advance via `Env.Advance(n)` to test jail expiry and decay.
func NewEnv() *Env {
	storeKey := storetypes.NewKVStoreKey(workertypes.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	_ = stateStore.LoadLatestVersion()
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	bank := NewMockBank()
	wk := workerkeeper.NewKeeper(cdc, storeKey, bank, log.NewNopLogger())
	startHeight := int64(100)
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: startHeight}, false, log.NewNopLogger())
	wk.SetParams(ctx, workertypes.DefaultParams())
	return &Env{
		Ctx:      ctx,
		Worker:   wk,
		Bank:     bank,
		height:   startHeight,
		maxBlock: startHeight,
	}
}

// Advance moves the block height forward by `blocks` and refreshes the context
// so any time-sensitive keeper logic (jail expiry, decay) sees the new height.
func (e *Env) Advance(blocks int64) {
	if blocks <= 0 {
		return
	}
	e.height += blocks
	if e.height > e.maxBlock {
		e.maxBlock = e.height
	}
	e.Ctx = e.Ctx.WithBlockHeight(e.height)
}

// Height returns the current block height. Used by invariants and assertions.
func (e *Env) Height() int64 { return e.height }

// MaxHeight is the highest block this round reached. Useful for the jail-expiry
// invariant: a worker's JailUntil should never exceed maxBlock + Jail2Duration.
func (e *Env) MaxHeight() int64 { return e.maxBlock }

// MakeWorker constructs and persists a fresh active worker with a random suffix.
//
// `stakeUFAI` is the initial stake in uFAI. The address is deterministic from
// `idx` so different scenarios can refer to "worker 0", "worker 1", etc. across
// rounds without name clashes.
func (e *Env) MakeWorker(idx int, stakeUFAI int64) sdk.AccAddress {
	addr := DerivAddr(idx)
	w := workertypes.Worker{
		Address:         addr.String(),
		Pubkey:          fmt.Sprintf("pubkey-%d", idx),
		Stake:           sdk.NewCoin("ufai", math.NewInt(stakeUFAI)),
		SupportedModels: []string{"model1"},
		Status:          workertypes.WorkerStatusActive,
		JoinedAt:        e.height,
		Endpoint:        fmt.Sprintf("worker-%d.local:8080", idx),
		GpuModel:        "H100",
		GpuVramGb:       80,
		GpuCount:        1,
		OperatorId:      fmt.Sprintf("op-%d", idx),
		TotalFeeEarned:  sdk.NewCoin("ufai", math.ZeroInt()),
		// 10000 = 1.0 effective reputation. Storing it explicitly (rather than
		// relying on EffectiveReputation's "0 → 10000" fallback) keeps the
		// invariant assertion on ReputationScore unambiguous.
		ReputationScore: workertypes.ReputationInitial,
	}
	e.Worker.SetWorker(e.Ctx, w)
	return addr
}

// DerivAddr deterministically maps an integer index to a 20-byte address.
// Same index → same address across rounds, so scenario code stays simple.
func DerivAddr(idx int) sdk.AccAddress {
	bz := make([]byte, 20)
	bz[0] = byte(idx & 0xFF)
	bz[1] = byte((idx >> 8) & 0xFF)
	return sdk.AccAddress(bz)
}

// MustGet returns the worker or panics with the scenario context.
//
// Used in invariant checks where the worker must exist by construction; if it
// does not, that itself is a state-machine bug the harness should surface.
func (e *Env) MustGet(addr sdk.AccAddress) workertypes.Worker {
	w, ok := e.Worker.GetWorker(e.Ctx, addr)
	if !ok {
		panic(fmt.Sprintf("worker not found: %s", addr.String()))
	}
	return w
}

// RoundResult is one row in the §6 report.
type RoundResult struct {
	ScenarioID  string
	Tier        Tier
	Round       int
	Seed        int64
	Err         error    // nil = PASS
	Invariants  []string // names of invariants that failed (empty = all PASS)
	StateDigest string   // optional human-readable snapshot for failed runs
}

// Pass reports whether this round was clean (scenario PASS + all invariants PASS).
func (r *RoundResult) Pass() bool {
	return r.Err == nil && len(r.Invariants) == 0
}

// Run executes a single scenario for `rounds` rounds and returns the result set.
//
// Stops at the first failure if `failFast` is true (the default for `-test.short`
// runs and CI quick-runs). Otherwise collects every result so the report can
// show which rounds failed without halting on the first one.
func Run(t *testing.T, s Scenario, rounds int, failFast bool) []RoundResult {
	t.Helper()
	out := make([]RoundResult, 0, rounds)
	for i := 0; i < rounds; i++ {
		seed := int64(i)
		env := NewEnv()
		rng := rand.New(rand.NewSource(seed))
		res := RoundResult{
			ScenarioID: s.ID(),
			Tier:       s.Tier(),
			Round:      i,
			Seed:       seed,
		}
		res.Err = s.Run(env, rng)
		if res.Err == nil {
			res.Invariants = CheckAll(env)
		}
		out = append(out, res)
		if failFast && !res.Pass() {
			t.Errorf("[%s] round=%d seed=%d FAIL: scenario_err=%v invariant_fails=%v",
				s.ID(), i, seed, res.Err, res.Invariants)
			return out
		}
	}
	return out
}
