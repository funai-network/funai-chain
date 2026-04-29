package keeper_test

// Tests for KT 30-case Issue 1 — per-request accept-time chain-side reservation.
//
// Pre-fix: per-request billing had no chain-side balance freeze at task accept
// time. A user could deposit, dispatch a task with fee=F, then withdraw down
// to balance < F before BatchSettlement landed; the keeper's settlement
// fallback would silently REFUNDED the entry and the Worker ate the cost.
//
// Per-token billing already had FreezeBalance/UnfreezeBalance from
// MsgRequestQuote; this PR adds the equivalent entry point for per-request
// via MsgBatchReserve. ProcessBatchSettlement and settleAuditedTask now
// release the freeze unconditionally per entry (idempotent UnfreezeBalance).
//
// The tests below pin both halves:
//   - reservation creates the freeze and Withdraw honors it
//   - settle releases the freeze, even on the per-request path
//   - bad rows skipped silently, batch not rejected
//   - merkle root mismatch jails proposer (mirrors BatchSettlement)

import (
	"crypto/sha256"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/keeper"
	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

// makeReserveMsg builds a properly-signed MsgBatchReserve for tests, mirroring
// makeBatchMsg for MsgBatchSettlement (keeper_test.go:170).
func makeReserveMsg(t *testing.T, proposer string, entries []types.ReserveEntry) *types.MsgBatchReserve {
	t.Helper()

	merkleRoot := keeper.ComputeReserveMerkleRoot(entries)
	msgHash := sha256.Sum256(merkleRoot)
	sig, err := testProposerKey.Sign(msgHash[:])
	if err != nil {
		t.Fatalf("sign reserve merkle root: %v", err)
	}

	return types.NewMsgBatchReserve(proposer, merkleRoot, entries, sig)
}

// ============================================================
// KT-Issue1-A. Reservation creates a freeze that Withdraw must honor.
// ============================================================

func TestKT_Issue1_ReserveBlocksWithdraw(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("kt-i1a-user")
	_ = k.ProcessDeposit(ctx, user, sdk.NewCoin("ufai", math.NewInt(1_000_000)))

	taskId := []byte("kt-i1a-task-00000001")
	maxFee := sdk.NewCoin("ufai", math.NewInt(800_000))

	entries := []types.ReserveEntry{
		{UserAddress: user.String(), TaskId: taskId, MaxFee: maxFee, ExpireBlock: 10000},
	}
	msg := makeReserveMsg(t, makeAddr("proposer").String(), entries)

	accepted, rejected, err := k.ProcessBatchReserve(ctx, msg)
	if err != nil {
		t.Fatalf("KT-Issue1-A: reserve should succeed: %v", err)
	}
	if accepted != 1 || rejected != 0 {
		t.Fatalf("KT-Issue1-A: expected 1 accepted 0 rejected, got %d/%d", accepted, rejected)
	}

	// FrozenBalance now 800k → AvailableBalance = 200k.
	ia, _ := k.GetInferenceAccount(ctx, user)
	if !ia.FrozenBalance.Amount.Equal(math.NewInt(800_000)) {
		t.Fatalf("KT-Issue1-A: expected FrozenBalance=800k, got %s", ia.FrozenBalance.Amount)
	}
	if !ia.AvailableBalance().Amount.Equal(math.NewInt(200_000)) {
		t.Fatalf("KT-Issue1-A: expected AvailableBalance=200k, got %s", ia.AvailableBalance().Amount)
	}

	// Withdraw of 200k succeeds (= AvailableBalance).
	if err := k.ProcessWithdraw(ctx, user, sdk.NewCoin("ufai", math.NewInt(200_000))); err != nil {
		t.Fatalf("KT-Issue1-A: withdraw of available 200k should succeed: %v", err)
	}

	// Withdraw of 1 more ufai fails — frozen amount is honored.
	if err := k.ProcessWithdraw(ctx, user, sdk.NewCoin("ufai", math.NewInt(1))); err == nil {
		t.Fatal("KT-Issue1-A: withdraw beyond AvailableBalance MUST fail when reservation is active")
	}
}

// ============================================================
// KT-Issue1-B. BatchSettlement releases the per-request freeze (the canonical
// race-fix). Without this, per-request settle would charge the fee but leave
// the freeze in place — locking the same amount twice.
// ============================================================

func TestKT_Issue1_BatchSettlementReleasesPerRequestFreeze(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	user := makeAddr("kt-i1b-user")
	worker := makeAddr("kt-i1b-worker")
	_ = k.ProcessDeposit(ctx, user, sdk.NewCoin("ufai", math.NewInt(5_000_000)))

	taskId := []byte("kt-i1b-task-00000001")
	maxFee := sdk.NewCoin("ufai", math.NewInt(2_000_000))
	settleFee := sdk.NewCoin("ufai", math.NewInt(1_500_000)) // actual ≤ max

	// Reserve.
	resMsg := makeReserveMsg(t, makeAddr("proposer").String(), []types.ReserveEntry{
		{UserAddress: user.String(), TaskId: taskId, MaxFee: maxFee, ExpireBlock: 10000},
	})
	if _, _, err := k.ProcessBatchReserve(ctx, resMsg); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	ia, _ := k.GetInferenceAccount(ctx, user)
	if !ia.FrozenBalance.Amount.Equal(math.NewInt(2_000_000)) {
		t.Fatalf("after reserve expected FrozenBalance=2M, got %s", ia.FrozenBalance.Amount)
	}

	// Settle.
	verifiers := []types.VerifierResult{
		{Address: makeAddr("kt-i1b-v1").String(), Pass: true},
		{Address: makeAddr("kt-i1b-v2").String(), Pass: true},
		{Address: makeAddr("kt-i1b-v3").String(), Pass: true},
	}
	settleEntries := []types.SettlementEntry{
		{TaskId: taskId, UserAddress: user.String(), WorkerAddress: worker.String(), Fee: settleFee, ExpireBlock: 10000, Status: types.SettlementSuccess, VerifierResults: verifiers},
	}
	settleMsg := makeBatchMsg(t, makeAddr("proposer").String(), settleEntries)
	if _, err := k.ProcessBatchSettlement(ctx, settleMsg); err != nil {
		t.Fatalf("settle: %v", err)
	}

	ia, _ = k.GetInferenceAccount(ctx, user)
	// Balance: 5_000_000 - 1_500_000 = 3_500_000.
	if !ia.Balance.Amount.Equal(math.NewInt(3_500_000)) {
		t.Fatalf("KT-Issue1-B: expected post-settle balance 3_500_000, got %s", ia.Balance.Amount)
	}
	// FrozenBalance: must be back to 0 — UnfreezeBalance ran for per-request.
	if !ia.FrozenBalance.IsZero() {
		t.Fatalf("KT-Issue1-B: expected FrozenBalance=0 after settle, got %s — per-request unfreeze regressed",
			ia.FrozenBalance)
	}
	// AvailableBalance: full 3_500_000 should be withdrawable.
	if !ia.AvailableBalance().Amount.Equal(math.NewInt(3_500_000)) {
		t.Fatalf("KT-Issue1-B: expected AvailableBalance=3_500_000, got %s", ia.AvailableBalance().Amount)
	}
	if err := k.ProcessWithdraw(ctx, user, sdk.NewCoin("ufai", math.NewInt(3_500_000))); err != nil {
		t.Fatalf("KT-Issue1-B: full withdraw post-settle should succeed: %v", err)
	}
}

// ============================================================
// KT-Issue1-C. Defensive — the original attack scenario.
// User deposits 1M, reserve freezes 800k, attacker tries to withdraw 950k
// (which would leave balance at 50k < fee). With the freeze, withdraw fails.
// Without the freeze (pre-fix), withdraw would succeed and Worker would
// eat the cost on the subsequent settle's REFUNDED fallback.
// ============================================================

func TestKT_Issue1_AttackScenario_WithdrawRaceBlocked(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("kt-i1c-greedy-user")
	_ = k.ProcessDeposit(ctx, user, sdk.NewCoin("ufai", math.NewInt(1_000_000)))

	taskId := []byte("kt-i1c-attack-task01")

	// Worker dispatches task with max_fee=800k → Leader submits reserve.
	resMsg := makeReserveMsg(t, makeAddr("proposer").String(), []types.ReserveEntry{
		{UserAddress: user.String(), TaskId: taskId, MaxFee: sdk.NewCoin("ufai", math.NewInt(800_000)), ExpireBlock: 10000},
	})
	if _, _, err := k.ProcessBatchReserve(ctx, resMsg); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// Attacker (= same user) tries to withdraw 950k. AvailableBalance = 200k.
	// Without the fix this succeeded; with the fix it must fail.
	err := k.ProcessWithdraw(ctx, user, sdk.NewCoin("ufai", math.NewInt(950_000)))
	if err == nil {
		t.Fatal("KT-Issue1-C: withdraw race attack must be blocked by reservation")
	}

	// Confirm: only AvailableBalance can be withdrawn.
	if err := k.ProcessWithdraw(ctx, user, sdk.NewCoin("ufai", math.NewInt(200_000))); err != nil {
		t.Fatalf("KT-Issue1-C: withdraw of AvailableBalance must still succeed: %v", err)
	}
	ia, _ := k.GetInferenceAccount(ctx, user)
	if !ia.Balance.Amount.Equal(math.NewInt(800_000)) {
		t.Fatalf("KT-Issue1-C: expected balance 800k after partial withdraw, got %s", ia.Balance.Amount)
	}
	if !ia.AvailableBalance().IsZero() {
		t.Fatalf("KT-Issue1-C: expected AvailableBalance=0 after partial withdraw, got %s", ia.AvailableBalance())
	}
}

// ============================================================
// KT-Issue1-D. Per-row silent skip on bad data — batch not rejected.
// Mirrors ProcessBatchSettlement's per-entry skip semantics.
// ============================================================

func TestKT_Issue1_PerRowSkip_BadDataDoesNotRejectBatch(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	good := makeAddr("kt-i1d-good-user")
	_ = k.ProcessDeposit(ctx, good, sdk.NewCoin("ufai", math.NewInt(10_000_000)))

	// Note: noAccount user has NEVER been deposited → GetInferenceAccount
	// returns found=false → entry silently rejected.
	noAccount := makeAddr("kt-i1d-no-account")

	// Insufficient: tiny deposit, too small to cover the requested freeze.
	poor := makeAddr("kt-i1d-poor-user")
	_ = k.ProcessDeposit(ctx, poor, sdk.NewCoin("ufai", math.NewInt(50)))

	entries := []types.ReserveEntry{
		// (1) good — should be accepted.
		{UserAddress: good.String(), TaskId: []byte("kt-i1d-task-good00001"), MaxFee: sdk.NewCoin("ufai", math.NewInt(1_000_000)), ExpireBlock: 10000},
		// (2) bad address — silent reject.
		{UserAddress: "not-a-bech32", TaskId: []byte("kt-i1d-task-badaddr01"), MaxFee: sdk.NewCoin("ufai", math.NewInt(1_000)), ExpireBlock: 10000},
		// (3) account missing — silent reject.
		{UserAddress: noAccount.String(), TaskId: []byte("kt-i1d-task-noacct001"), MaxFee: sdk.NewCoin("ufai", math.NewInt(1_000)), ExpireBlock: 10000},
		// (4) wrong denom — silent reject.
		{UserAddress: good.String(), TaskId: []byte("kt-i1d-task-baddenom1"), MaxFee: sdk.NewCoin("notufai", math.NewInt(1_000)), ExpireBlock: 10000},
		// (5) past expiry — silent reject.
		{UserAddress: good.String(), TaskId: []byte("kt-i1d-task-expired01"), MaxFee: sdk.NewCoin("ufai", math.NewInt(1_000)), ExpireBlock: 50},
		// (6) insufficient available balance — silent reject.
		{UserAddress: poor.String(), TaskId: []byte("kt-i1d-task-poorbal01"), MaxFee: sdk.NewCoin("ufai", math.NewInt(100_000)), ExpireBlock: 10000},
		// (7) zero max_fee — silent reject.
		{UserAddress: good.String(), TaskId: []byte("kt-i1d-task-zerofee01"), MaxFee: sdk.NewCoin("ufai", math.ZeroInt()), ExpireBlock: 10000},
	}

	msg := makeReserveMsg(t, makeAddr("proposer").String(), entries)
	accepted, rejected, err := k.ProcessBatchReserve(ctx, msg)
	if err != nil {
		t.Fatalf("KT-Issue1-D: per-row issues must not error the batch: %v", err)
	}
	if accepted != 1 {
		t.Fatalf("KT-Issue1-D: expected 1 accepted (the good row), got %d", accepted)
	}
	if rejected != 6 {
		t.Fatalf("KT-Issue1-D: expected 6 rejected, got %d", rejected)
	}

	// Only the good user's freeze should exist.
	iaGood, _ := k.GetInferenceAccount(ctx, good)
	if !iaGood.FrozenBalance.Amount.Equal(math.NewInt(1_000_000)) {
		t.Fatalf("KT-Issue1-D: good user FrozenBalance expected 1M, got %s", iaGood.FrozenBalance.Amount)
	}
	iaPoor, _ := k.GetInferenceAccount(ctx, poor)
	if !iaPoor.FrozenBalance.IsZero() {
		t.Fatalf("KT-Issue1-D: poor user MUST NOT be frozen, got %s", iaPoor.FrozenBalance)
	}
}

// ============================================================
// KT-Issue1-E. Duplicate reservation on same task is silently rejected.
// The first reserve creates the freeze; the second returns rejected=1 without
// double-charging the user. This is critical for Leader retries and
// crash-recovery.
// ============================================================

func TestKT_Issue1_DuplicateReserve_SilentReject(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("kt-i1e-user")
	_ = k.ProcessDeposit(ctx, user, sdk.NewCoin("ufai", math.NewInt(10_000_000)))

	taskId := []byte("kt-i1e-dup-task-00001")
	entry := types.ReserveEntry{
		UserAddress: user.String(),
		TaskId:      taskId,
		MaxFee:      sdk.NewCoin("ufai", math.NewInt(1_000_000)),
		ExpireBlock: 10000,
	}

	// First reserve: accepted.
	if _, _, err := k.ProcessBatchReserve(ctx,
		makeReserveMsg(t, makeAddr("proposer").String(), []types.ReserveEntry{entry})); err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	ia, _ := k.GetInferenceAccount(ctx, user)
	if !ia.FrozenBalance.Amount.Equal(math.NewInt(1_000_000)) {
		t.Fatalf("after 1st reserve expected FrozenBalance=1M, got %s", ia.FrozenBalance.Amount)
	}

	// Second reserve, same task: silent reject, no double-freeze.
	accepted, rejected, err := k.ProcessBatchReserve(ctx,
		makeReserveMsg(t, makeAddr("proposer").String(), []types.ReserveEntry{entry}))
	if err != nil {
		t.Fatalf("second reserve must not error: %v", err)
	}
	if accepted != 0 || rejected != 1 {
		t.Fatalf("KT-Issue1-E: expected 0 accepted 1 rejected, got %d/%d", accepted, rejected)
	}
	ia, _ = k.GetInferenceAccount(ctx, user)
	if !ia.FrozenBalance.Amount.Equal(math.NewInt(1_000_000)) {
		t.Fatalf("KT-Issue1-E: FrozenBalance must remain 1M after duplicate (no double-freeze), got %s",
			ia.FrozenBalance.Amount)
	}
}

// ============================================================
// KT-Issue1-F. Multiple users / multiple tasks accumulate correctly.
// ============================================================

func TestKT_Issue1_MultipleReservesAccumulate(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	alice := makeAddr("kt-i1f-alice")
	bob := makeAddr("kt-i1f-bob")
	_ = k.ProcessDeposit(ctx, alice, sdk.NewCoin("ufai", math.NewInt(10_000_000)))
	_ = k.ProcessDeposit(ctx, bob, sdk.NewCoin("ufai", math.NewInt(5_000_000)))

	entries := []types.ReserveEntry{
		{UserAddress: alice.String(), TaskId: []byte("kt-i1f-alice-task1xx"), MaxFee: sdk.NewCoin("ufai", math.NewInt(2_000_000)), ExpireBlock: 10000},
		{UserAddress: alice.String(), TaskId: []byte("kt-i1f-alice-task2xx"), MaxFee: sdk.NewCoin("ufai", math.NewInt(3_000_000)), ExpireBlock: 10000},
		{UserAddress: bob.String(), TaskId: []byte("kt-i1f-bob-task-1-xxx"), MaxFee: sdk.NewCoin("ufai", math.NewInt(1_000_000)), ExpireBlock: 10000},
	}
	msg := makeReserveMsg(t, makeAddr("proposer").String(), entries)
	accepted, rejected, err := k.ProcessBatchReserve(ctx, msg)
	if err != nil {
		t.Fatalf("multi-user reserve: %v", err)
	}
	if accepted != 3 || rejected != 0 {
		t.Fatalf("KT-Issue1-F: expected 3 accepted 0 rejected, got %d/%d", accepted, rejected)
	}

	iaAlice, _ := k.GetInferenceAccount(ctx, alice)
	if !iaAlice.FrozenBalance.Amount.Equal(math.NewInt(5_000_000)) {
		t.Fatalf("KT-Issue1-F: alice expected FrozenBalance=5M (2M+3M), got %s", iaAlice.FrozenBalance.Amount)
	}
	iaBob, _ := k.GetInferenceAccount(ctx, bob)
	if !iaBob.FrozenBalance.Amount.Equal(math.NewInt(1_000_000)) {
		t.Fatalf("KT-Issue1-F: bob expected FrozenBalance=1M, got %s", iaBob.FrozenBalance.Amount)
	}
}

// ============================================================
// KT-Issue1-G. Merkle root mismatch jails proposer (mirrors BatchSettlement
// security posture). Tampering with entries post-sign must be detected.
// ============================================================

func TestKT_Issue1_MerkleRootMismatch_JailsProposer(t *testing.T) {
	k, ctx, _, wk := setupKeeper(t)

	user := makeAddr("kt-i1g-user")
	_ = k.ProcessDeposit(ctx, user, sdk.NewCoin("ufai", math.NewInt(10_000_000)))

	originalEntries := []types.ReserveEntry{
		{UserAddress: user.String(), TaskId: []byte("kt-i1g-orig-task00001"), MaxFee: sdk.NewCoin("ufai", math.NewInt(1_000_000)), ExpireBlock: 10000},
	}

	// Build msg with original entries; then tamper the entries so the
	// embedded merkle root + signature no longer match.
	msg := makeReserveMsg(t, makeAddr("proposer").String(), originalEntries)
	msg.Entries = []types.ReserveEntry{
		{UserAddress: user.String(), TaskId: []byte("kt-i1g-tampered-task1"), MaxFee: sdk.NewCoin("ufai", math.NewInt(9_000_000)), ExpireBlock: 10000},
	}
	msg.ResultCount = uint32(len(msg.Entries))

	_, _, err := k.ProcessBatchReserve(ctx, msg)
	if err == nil {
		t.Fatal("KT-Issue1-G: tampered batch must error")
	}

	// Per-spec: merkle mismatch → proposer jailed.
	if len(wk.jailCalls) != 1 {
		t.Fatalf("KT-Issue1-G: expected exactly 1 jail call on proposer, got %d", len(wk.jailCalls))
	}
}

// ============================================================
// KT-Issue1-H. Empty entries list rejected via ValidateBasic.
// ============================================================

func TestKT_Issue1_ValidateBasic_EmptyEntries(t *testing.T) {
	msg := types.NewMsgBatchReserve(
		makeAddr("kt-i1h-prop").String(),
		[]byte("merkleroot"),
		[]types.ReserveEntry{},
		[]byte("sig"),
	)
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("KT-Issue1-H: empty entries must fail ValidateBasic")
	}
}
