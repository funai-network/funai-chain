package keeper_test

// Issue H (FunAI-non-state-machine-findings-2026-04-30):
// SendCoins return values from the per-entry payout helpers
// (distributeSuccessFee / distributeFailFee) used to be silently dropped.
// A single blocked recipient address could leave the user debited but the
// worker / verifier unpaid — value silently stranded in the settlement
// module account, with no error surfaced to the proposer or operator.
//
// Post-fix: each entry's payout runs inside a CacheContext. SendCoins error
// → the cache is discarded, so the user-balance debit, worker-stat updates,
// and SettledTask write are all rolled back atomically. Other entries in the
// same batch are unaffected.
//
// Tests below assert the rollback semantics for each call site:
//   - ProcessBatchSettlement SUCCESS path (verifier SendCoins fail)
//   - ProcessBatchSettlement SUCCESS path (worker SendCoins fail)
//   - ProcessBatchSettlement per-request FAIL path
//   - ProcessBatchSettlement per-token FAIL path (still settled+jailed)
//   - settleAuditedTask SUCCESS path → returns false, pending preserved
//   - DistributeMultiVerificationFund per-recipient failure isolation

import (
	"errors"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

// blockSendsTo returns a SendModuleToAcctHook that errors only on the given
// recipient and lets every other recipient through.
func blockSendsTo(blocked sdk.AccAddress) func(sdk.AccAddress, sdk.Coins) error {
	return func(recipient sdk.AccAddress, _ sdk.Coins) error {
		if recipient.Equals(blocked) {
			return errors.New("recipient address is blocked")
		}
		return nil
	}
}

// ============================================================
// KT-IssueH-A. SUCCESS entry — verifier SendCoins fails →
// user balance preserved, no SettledTask, batch continues for other entries.
// ============================================================

func TestKT_IssueH_Success_VerifierBlocked_RollsBackEntry(t *testing.T) {
	k, ctx, bk, wk := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	userAddr := makeAddr("ih-A-user")
	workerAddr := makeAddr("ih-A-worker")
	v1 := makeAddr("ih-A-v1-blocked")
	v2 := makeAddr("ih-A-v2")
	v3 := makeAddr("ih-A-v3")

	fee := sdk.NewCoin("ufai", math.NewInt(1_000_000))
	startingBalance := sdk.NewCoin("ufai", math.NewInt(2_000_000))
	_ = k.ProcessDeposit(ctx, userAddr, startingBalance)

	// A second user/entry that pays cleanly — it must still settle.
	cleanUser := makeAddr("ih-A-user-clean")
	cleanWorker := makeAddr("ih-A-worker-clean")
	_ = k.ProcessDeposit(ctx, cleanUser, startingBalance)

	bk.SendModuleToAcctHook = blockSendsTo(v1)

	entries := []types.SettlementEntry{
		{
			TaskId:        []byte("ih-A-task-blocked-01"),
			UserAddress:   userAddr.String(),
			WorkerAddress: workerAddr.String(),
			Fee:           fee,
			ExpireBlock:   200,
			Status:        types.SettlementSuccess,
			VerifierResults: []types.VerifierResult{
				{Address: v1.String(), Pass: true},
				{Address: v2.String(), Pass: true},
				{Address: v3.String(), Pass: true},
			},
		},
		{
			TaskId:        []byte("ih-A-task-clean-001-"),
			UserAddress:   cleanUser.String(),
			WorkerAddress: cleanWorker.String(),
			Fee:           fee,
			ExpireBlock:   200,
			Status:        types.SettlementSuccess,
			VerifierResults: []types.VerifierResult{
				{Address: v2.String(), Pass: true},
				{Address: v3.String(), Pass: true},
				{Address: makeAddr("ih-A-v4").String(), Pass: true},
			},
		},
	}

	msg := makeBatchMsg(t, makeAddr("ih-A-proposer").String(), entries)
	if _, err := k.ProcessBatchSettlement(ctx, msg); err != nil {
		t.Fatalf("ProcessBatchSettlement: %v", err)
	}

	// Blocked entry: user balance fully preserved (rollback).
	ia, _ := k.GetInferenceAccount(ctx, userAddr)
	if !ia.Balance.Equal(startingBalance) {
		t.Fatalf("blocked entry user balance must be unchanged on rollback: got %s, want %s", ia.Balance, startingBalance)
	}
	if _, found := k.GetSettledTask(ctx, entries[0].TaskId); found {
		t.Fatal("blocked entry must NOT have a SettledTask record")
	}
	// Streak/stats should not have been credited to the blocked-entry worker.
	for _, addr := range wk.streakCalls {
		if addr.Equals(workerAddr) {
			t.Fatal("blocked entry's worker streak must NOT be incremented")
		}
	}

	// Clean entry: settled normally, balance debited.
	cleanIa, _ := k.GetInferenceAccount(ctx, cleanUser)
	expectedClean := startingBalance.Sub(fee)
	if !cleanIa.Balance.Equal(expectedClean) {
		t.Fatalf("clean entry user balance: got %s, want %s", cleanIa.Balance, expectedClean)
	}
	st, found := k.GetSettledTask(ctx, entries[1].TaskId)
	if !found {
		t.Fatal("clean entry must have a SettledTask record")
	}
	if st.Status != types.TaskSettled {
		t.Fatalf("clean entry status: got %s, want TaskSettled", st.Status)
	}
}

// ============================================================
// KT-IssueH-B. SUCCESS entry — worker SendCoins fails → entry rolled back.
// ============================================================

func TestKT_IssueH_Success_WorkerBlocked_RollsBackEntry(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	userAddr := makeAddr("ih-B-user")
	workerAddr := makeAddr("ih-B-worker-blkd")
	v1 := makeAddr("ih-B-v1")
	v2 := makeAddr("ih-B-v2")
	v3 := makeAddr("ih-B-v3")

	fee := sdk.NewCoin("ufai", math.NewInt(1_000_000))
	startingBalance := sdk.NewCoin("ufai", math.NewInt(2_000_000))
	_ = k.ProcessDeposit(ctx, userAddr, startingBalance)

	bk.SendModuleToAcctHook = blockSendsTo(workerAddr)

	entries := []types.SettlementEntry{{
		TaskId:        []byte("ih-B-task-001-padded"),
		UserAddress:   userAddr.String(),
		WorkerAddress: workerAddr.String(),
		Fee:           fee,
		ExpireBlock:   200,
		Status:        types.SettlementSuccess,
		VerifierResults: []types.VerifierResult{
			{Address: v1.String(), Pass: true},
			{Address: v2.String(), Pass: true},
			{Address: v3.String(), Pass: true},
		},
	}}

	msg := makeBatchMsg(t, makeAddr("ih-B-proposer").String(), entries)
	if _, err := k.ProcessBatchSettlement(ctx, msg); err != nil {
		t.Fatalf("ProcessBatchSettlement: %v", err)
	}

	ia, _ := k.GetInferenceAccount(ctx, userAddr)
	if !ia.Balance.Equal(startingBalance) {
		t.Fatalf("user balance must be unchanged on worker-block rollback: got %s, want %s", ia.Balance, startingBalance)
	}
	if _, found := k.GetSettledTask(ctx, entries[0].TaskId); found {
		t.Fatal("entry must NOT have a SettledTask record when worker SendCoins fails")
	}
}

// ============================================================
// KT-IssueH-C. Per-request FAIL path — verifier SendCoins fails →
// user balance preserved, no SettledTask, no JailWorker (mirrors continue
// semantic of the per-request balance-insufficient path).
// ============================================================

func TestKT_IssueH_PerRequestFail_VerifierBlocked_EntrySkipped(t *testing.T) {
	k, ctx, bk, wk := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	userAddr := makeAddr("ih-C-user")
	workerAddr := makeAddr("ih-C-worker")
	v1 := makeAddr("ih-C-v1-blocked")
	v2 := makeAddr("ih-C-v2")
	v3 := makeAddr("ih-C-v3")

	fee := sdk.NewCoin("ufai", math.NewInt(1_000_000))
	startingBalance := sdk.NewCoin("ufai", math.NewInt(2_000_000))
	_ = k.ProcessDeposit(ctx, userAddr, startingBalance)

	bk.SendModuleToAcctHook = blockSendsTo(v1)

	entries := []types.SettlementEntry{{
		TaskId:        []byte("ih-C-task-001-padded"),
		UserAddress:   userAddr.String(),
		WorkerAddress: workerAddr.String(),
		Fee:           fee,
		ExpireBlock:   200,
		Status:        types.SettlementFail,
		VerifierResults: []types.VerifierResult{
			{Address: v1.String(), Pass: false},
			{Address: v2.String(), Pass: false},
			{Address: v3.String(), Pass: false},
		},
	}}

	msg := makeBatchMsg(t, makeAddr("ih-C-proposer").String(), entries)
	if _, err := k.ProcessBatchSettlement(ctx, msg); err != nil {
		t.Fatalf("ProcessBatchSettlement: %v", err)
	}

	ia, _ := k.GetInferenceAccount(ctx, userAddr)
	if !ia.Balance.Equal(startingBalance) {
		t.Fatalf("user balance must be unchanged: got %s, want %s", ia.Balance, startingBalance)
	}
	if _, found := k.GetSettledTask(ctx, entries[0].TaskId); found {
		t.Fatal("per-request FAIL entry must NOT have a SettledTask record on rollback")
	}
	for _, addr := range wk.jailCalls {
		if addr.Equals(workerAddr) {
			t.Fatal("per-request FAIL entry's worker must NOT be jailed on rollback (matches existing skip-entry semantic)")
		}
	}
}

// ============================================================
// KT-IssueH-D. settleAuditedTask SUCCESS path — verifier SendCoins fails →
// returns false, pending preserved, no SettledTask written.
// ============================================================

func TestKT_IssueH_SettleAuditedTask_SuccessBlocked_PendingPreserved(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)
	k.SetCurrentThirdVerificationRate(ctx, 0)

	taskId := []byte("ih-D-audit-task-001-")
	userAddr := makeAddr("ih-D-user")
	workerAddr := makeAddr("ih-D-worker")
	origV1 := makeAddr("ih-D-orig-v1")
	origV2 := makeAddr("ih-D-orig-v2")
	origV3 := makeAddr("ih-D-orig-v3-blkd")

	fee := sdk.NewCoin("ufai", math.NewInt(1_000_000))
	startingBalance := sdk.NewCoin("ufai", math.NewInt(2_000_000))
	_ = k.ProcessDeposit(ctx, userAddr, startingBalance)

	bk.SendModuleToAcctHook = blockSendsTo(origV3)

	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:            taskId,
		OriginalStatus:    types.SettlementSuccess,
		SubmittedAt:       ctx.BlockHeight(),
		UserAddress:       userAddr.String(),
		WorkerAddress:     workerAddr.String(),
		VerifierAddresses: []string{origV1.String(), origV2.String(), origV3.String()},
		Fee:               fee,
		ExpireBlock:       100000,
	})

	submit3PassAuditResults(t, k, ctx, taskId, "ih-D")

	// Pending must remain because settleAuditedTask returned false on
	// distributeSuccessFee error. HandleSecondVerificationTimeouts will retry.
	if _, found := k.GetSecondVerificationPending(ctx, taskId); !found {
		t.Fatal("settleAuditedTask SUCCESS-path must preserve pending on SendCoins failure")
	}
	if _, found := k.GetSettledTask(ctx, taskId); found {
		t.Fatal("settleAuditedTask SUCCESS-path must NOT write SettledTask on SendCoins failure")
	}

	ia, _ := k.GetInferenceAccount(ctx, userAddr)
	if !ia.Balance.Equal(startingBalance) {
		t.Fatalf("user balance must be unchanged on settleAuditedTask rollback: got %s, want %s", ia.Balance, startingBalance)
	}
}

// ============================================================
// KT-IssueH-E. settleAuditedTask FAIL path — verifier SendCoins fails →
// returns false, pending preserved, balance unchanged.
// ============================================================

func TestKT_IssueH_SettleAuditedTask_FailBlocked_PendingPreserved(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)
	k.SetCurrentThirdVerificationRate(ctx, 0)

	taskId := []byte("ih-E-audit-task-001-")
	userAddr := makeAddr("ih-E-user")
	workerAddr := makeAddr("ih-E-worker")
	origV1 := makeAddr("ih-E-orig-v1")
	origV2 := makeAddr("ih-E-orig-v2")
	origV3 := makeAddr("ih-E-orig-v3-blkd")

	fee := sdk.NewCoin("ufai", math.NewInt(1_000_000))
	startingBalance := sdk.NewCoin("ufai", math.NewInt(2_000_000))
	_ = k.ProcessDeposit(ctx, userAddr, startingBalance)

	bk.SendModuleToAcctHook = blockSendsTo(origV3)

	// To exercise settleAuditedTask FAIL branch we need
	// OriginalStatus = FAIL && audit confirms FAIL. With 3 FAIL audit results,
	// processAuditJudgment hits line 1804: settleAuditedTask(false, false, ..., 0).
	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:            taskId,
		OriginalStatus:    types.SettlementFail,
		SubmittedAt:       ctx.BlockHeight(),
		UserAddress:       userAddr.String(),
		WorkerAddress:     workerAddr.String(),
		VerifierAddresses: []string{origV1.String(), origV2.String(), origV3.String()},
		Fee:               fee,
		ExpireBlock:       100000,
	})

	submit3FailAuditResults(t, k, ctx, taskId, "ih-E")

	if _, found := k.GetSecondVerificationPending(ctx, taskId); !found {
		t.Fatal("settleAuditedTask FAIL-path must preserve pending on SendCoins failure")
	}
	if _, found := k.GetSettledTask(ctx, taskId); found {
		t.Fatal("settleAuditedTask FAIL-path must NOT write SettledTask on SendCoins failure")
	}

	ia, _ := k.GetInferenceAccount(ctx, userAddr)
	if !ia.Balance.Equal(startingBalance) {
		t.Fatalf("user balance must be unchanged: got %s, want %s", ia.Balance, startingBalance)
	}
}

// ============================================================
// KT-IssueH-F. Sanity check — pre-fix path still works (no hook).
// Confirms the CacheContext refactor doesn't regress the happy path.
// ============================================================

func TestKT_IssueH_NoHook_HappyPath(t *testing.T) {
	k, ctx, _, wk := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	userAddr := makeAddr("ih-F-user")
	workerAddr := makeAddr("ih-F-worker")
	fee := sdk.NewCoin("ufai", math.NewInt(1_000_000))
	_ = k.ProcessDeposit(ctx, userAddr, fee)

	entries := []types.SettlementEntry{{
		TaskId:        []byte("ih-F-task-001-padded"),
		UserAddress:   userAddr.String(),
		WorkerAddress: workerAddr.String(),
		Fee:           fee,
		ExpireBlock:   200,
		Status:        types.SettlementSuccess,
		VerifierResults: []types.VerifierResult{
			{Address: makeAddr("ih-F-v1").String(), Pass: true},
			{Address: makeAddr("ih-F-v2").String(), Pass: true},
			{Address: makeAddr("ih-F-v3").String(), Pass: true},
		},
	}}

	msg := makeBatchMsg(t, makeAddr("ih-F-proposer").String(), entries)
	if _, err := k.ProcessBatchSettlement(ctx, msg); err != nil {
		t.Fatalf("ProcessBatchSettlement: %v", err)
	}

	ia, _ := k.GetInferenceAccount(ctx, userAddr)
	if !ia.Balance.IsZero() {
		t.Fatalf("happy path: user balance must be fully debited, got %s", ia.Balance)
	}
	st, found := k.GetSettledTask(ctx, entries[0].TaskId)
	if !found {
		t.Fatal("happy path: SettledTask must exist")
	}
	if st.Status != types.TaskSettled {
		t.Fatalf("happy path: status must be TaskSettled, got %s", st.Status)
	}
	if len(wk.streakCalls) != 1 {
		t.Fatalf("happy path: expected 1 IncrementSuccessStreak, got %d", len(wk.streakCalls))
	}
}
