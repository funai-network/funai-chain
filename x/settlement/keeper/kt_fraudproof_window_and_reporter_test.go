package keeper_test

// Tests for FraudProof Phase 1: time window + reporter binding.
// These close two gaps in the original V5.2 §12.4 design that an external
// review surfaced (docs/testing/FraudProof最小安全改造方案):
//
//   - No time bound: historic SettledTask / SecondVerificationPending entries
//     remained indefinitely exposed to the slashing entry, with no signal to
//     workers about how long evidence must be retained.
//
//   - No reporter binding: any registered address holding a (ContentHash,
//     WorkerContentSig) pair could submit FraudProof on someone else's task.
//     The KT Issue 5 series fixed worker / footprint binding but left the
//     reporter unconstrained.
//
// Graceful migration: both checks no-op when the relevant footprint field is
// zero / empty (legacy state written before this change). The frozen-only
// path is exempt by design — see ProcessFraudProof comment.

import (
	"testing"

	"cosmossdk.io/math"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

// ============================================================
// Window check — SettledTask path
// ============================================================

func TestFraudProof_Window_SettledTask_AcceptsInsideWindow(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("fwspw-user")
	worker := makeAddr("fwspw-worker")
	taskId := []byte("fwspw-inside-window01")

	settledAt := ctx.BlockHeight()
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     settledAt,
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         user.String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("inside window must succeed: %v", err)
	}
	if !k.HasFraudMark(ctx, taskId) {
		t.Fatal("fraud mark must be set on accepted proof inside window")
	}
}

func TestFraudProof_Window_SettledTask_RejectsOutsideWindow(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("fwspw-user-2")
	worker := makeAddr("fwspw-worker-2")
	taskId := []byte("fwspw-outside-window1")

	settledAt := int64(50)
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     settledAt,
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	// Advance ctx height past SettledAt + FraudWindowBlocks (default 17280).
	farFuture := settledAt + int64(types.DefaultFraudWindowBlocks) + 1
	futureCtx := ctx.WithBlockHeader(cmtproto.Header{Height: farFuture})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         user.String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	err := k.ProcessFraudProof(futureCtx, msg)
	if err == nil {
		t.Fatal("outside window must reject")
	}
	if k.HasFraudMark(futureCtx, taskId) {
		t.Fatal("fraud mark must NOT be set when window expired")
	}
}

// Legacy migration: SettledTask written before this change has SettledAt=0;
// the window check must skip rather than reject (otherwise every legacy
// FraudProof tx would fail post-upgrade).
func TestFraudProof_Window_SettledTask_GracefulOnLegacyZeroSettledAt(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("fwspw-user-3")
	worker := makeAddr("fwspw-worker-3")
	taskId := []byte("fwspw-legacy-settledat")

	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     0, // legacy: written before SettledAt was wired up
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         user.String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("legacy SettledAt=0 must skip window check, got: %v", err)
	}
}

// ============================================================
// Window check — SecondVerificationPending path
// ============================================================

func TestFraudProof_Window_PendingAudit_RejectsOutsideWindow(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("fwspw-pa-user")
	worker := makeAddr("fwspw-pa-worker")
	taskId := []byte("fwspw-pa-outside-win01")

	submittedAt := int64(40)
	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:        taskId,
		SubmittedAt:   submittedAt,
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
		ExpireBlock:   100000,
	})

	farFuture := submittedAt + int64(types.DefaultFraudWindowBlocks) + 1
	futureCtx := ctx.WithBlockHeader(cmtproto.Header{Height: farFuture})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         user.String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	err := k.ProcessFraudProof(futureCtx, msg)
	if err == nil {
		t.Fatal("outside pending audit window must reject")
	}
}

// ============================================================
// Reporter binding — SettledTask path
// ============================================================

func TestFraudProof_ReporterBinding_SettledTask_RejectsMismatch(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("frbst-user")
	intruder := makeAddr("frbst-intruder")
	worker := makeAddr("frbst-worker")
	taskId := []byte("frbst-reporter-mismatch")

	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         intruder.String(), // not the task user
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	err := k.ProcessFraudProof(ctx, msg)
	if err == nil {
		t.Fatal("reporter ≠ task user must reject")
	}
	if k.HasFraudMark(ctx, taskId) {
		t.Fatal("fraud mark must NOT be set when reporter binding fails")
	}
}

func TestFraudProof_ReporterBinding_SettledTask_AcceptsMatch(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("frbst-match-user")
	worker := makeAddr("frbst-match-worker")
	taskId := []byte("frbst-reporter-match01")

	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         user.String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("matching reporter must succeed: %v", err)
	}
}

// Legacy migration: SettledTask written before UserAddress was populated. The
// reporter binding must skip rather than reject.
func TestFraudProof_ReporterBinding_GracefulOnLegacyEmptyUser(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	worker := makeAddr("frbst-legacy-worker")
	taskId := []byte("frbst-legacy-empty-user")

	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   "", // legacy
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("frbst-legacy-rep").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("legacy empty UserAddress must skip reporter binding, got: %v", err)
	}
}

// ============================================================
// Reporter binding — SecondVerificationPending path
// ============================================================

func TestFraudProof_ReporterBinding_PendingAudit_RejectsMismatch(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("frbpa-user")
	intruder := makeAddr("frbpa-intruder")
	worker := makeAddr("frbpa-worker")
	taskId := []byte("frbpa-reporter-mismatch")

	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:        taskId,
		SubmittedAt:   ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
		ExpireBlock:   100000,
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         intruder.String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	err := k.ProcessFraudProof(ctx, msg)
	if err == nil {
		t.Fatal("reporter ≠ task user (pending audit path) must reject")
	}
}

// ============================================================
// Regression: audit re-settle paths must populate UserAddress
// ============================================================

// External review on PR #53 flagged that the new reporter binding silently
// no-ops when the on-chain SettledTask has UserAddress="" (legacy migration
// path). For the binding to be effective post-upgrade, every newly-written
// SettledTask must carry UserAddress. This test pins the audit-re-settle
// path: SecondVerificationPending → 3 PASS results → settleAuditedTask
// SUCCESS branch → SettledTask. The resulting SettledTask MUST have
// UserAddress populated for FraudProof's reporter binding to work.
func TestFraudProof_AuditResettle_PopulatesUserAddress(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)
	k.SetCurrentThirdVerificationRate(ctx, 0)

	user := makeAddr("frar-user")
	worker := makeAddr("frar-worker")
	taskId := []byte("frar-audit-resettle01")

	_ = k.ProcessDeposit(ctx, user, sdk.NewCoin("ufai", math.NewInt(10_000_000)))

	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:            taskId,
		OriginalStatus:    types.SettlementSuccess,
		SubmittedAt:       ctx.BlockHeight(),
		WorkerAddress:     worker.String(),
		UserAddress:       user.String(),
		VerifierAddresses: []string{makeAddr("frar-orig-v1").String()},
		Fee:               sdk.NewCoin("ufai", math.NewInt(1_000_000)),
		ExpireBlock:       100000,
	})

	for i := 0; i < 3; i++ {
		err := k.ProcessSecondVerificationResult(ctx, &types.MsgSecondVerificationResult{
			SecondVerifier: makeAddr("frar-aud-v" + string(rune('0'+i))).String(),
			TaskId:         taskId,
			Epoch:          1,
			Pass:           true,
			LogitsHash:     []byte("h"),
		})
		if err != nil {
			t.Fatalf("audit result %d: %v", i, err)
		}
	}

	st, found := k.GetSettledTask(ctx, taskId)
	if !found {
		t.Fatal("SettledTask must exist after audit completion")
	}
	if st.UserAddress != user.String() {
		t.Fatalf("audit-resettled SettledTask must carry UserAddress=%s, got %q",
			user.String(), st.UserAddress)
	}
	if st.SettledAt == 0 {
		t.Fatal("audit-resettled SettledTask must carry non-zero SettledAt")
	}
}

// Sanity: when both SettledTask and SecondVerificationPending exist (a task
// that was settled into PendingSecondVerification status), the SettledTask
// binding wins. This mirrors the existing footprint resolution order.
func TestFraudProof_BothFootprints_SettledWins(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("frbf-user")
	worker := makeAddr("frbf-worker")
	taskId := []byte("frbf-both-footprints01")

	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskPendingSecondVerification,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})
	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:        taskId,
		SubmittedAt:   ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
		ExpireBlock:   100000,
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         user.String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("dual footprint with matching reporter must succeed: %v", err)
	}
}
