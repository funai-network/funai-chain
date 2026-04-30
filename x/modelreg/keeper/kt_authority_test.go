package keeper_test

// Tests for KT 30-case Issue 16 — modelreg authority gate + ComputeModelId
// boundary collision.
//
// Issue 16  MsgUpdateModelStats handler did not validate msg.Authority. Any
//           account with a valid bech32 address could overwrite a model's
//           InstalledStakeRatio / WorkerCount / OperatorCount, then trigger
//           CheckAndActivateModel with the manipulated values. Effect: anyone
//           could activate or deactivate arbitrary models.
//
// Issue 9   ComputeModelId concatenated weight/quant/runtime hashes with no
//           separator. Variable-length inputs allowed boundary collisions —
//           two different (W, Q, R) tuples could land on the same model_id.

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/modelreg/keeper"
	"github.com/funai-wiki/funai-chain/x/modelreg/types"
)

// ============================================================
// Issue 16 — UpdateModelStats authority gate.
// ============================================================

func TestKT_Issue16_UpdateModelStats_RejectsWrongAuthority(t *testing.T) {
	k, ctx, _ := setupModelregKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	// Pre-populate a proposed model so handler reaches the authority check.
	k.SetModel(ctx, types.Model{
		ModelId:        "issue16-wrong-auth",
		Name:           "wrong-auth-model",
		Epsilon:        100,
		Status:         types.ModelStatusProposed,
		SuggestedPrice: sdk.NewCoin("ufai", math.NewInt(100)),
	})

	// Submit MsgUpdateModelStats with a bech32 that is NOT the gov authority.
	attacker := sdk.AccAddress([]byte("kt-i16-attacker_____")).String()
	msg := types.NewMsgUpdateModelStats(attacker, "issue16-wrong-auth", 0.95, 99, 99)

	_, err := msgServer.UpdateModelStats(ctx, msg)
	if err == nil {
		t.Fatal("Issue 16: handler MUST reject when msg.Authority != gov authority")
	}
	// State must be untouched.
	m, _ := k.GetModel(ctx, "issue16-wrong-auth")
	if m.Status != types.ModelStatusProposed {
		t.Fatalf("Issue 16: model status must stay proposed, got %s", m.Status)
	}
	if m.WorkerCount != 0 {
		t.Fatalf("Issue 16: WorkerCount must stay 0, got %d", m.WorkerCount)
	}
}

func TestKT_Issue16_UpdateModelStats_AcceptsCorrectAuthority(t *testing.T) {
	// Sanity that the post-fix path still works for the legitimate authority.
	k, ctx, _ := setupModelregKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	k.SetModel(ctx, types.Model{
		ModelId:        "issue16-good-auth",
		Name:           "good-auth-model",
		Epsilon:        100,
		Status:         types.ModelStatusProposed,
		SuggestedPrice: sdk.NewCoin("ufai", math.NewInt(100)),
	})

	msg := types.NewMsgUpdateModelStats(testGovAuthority, "issue16-good-auth", 0.8, 5, 5)
	if _, err := msgServer.UpdateModelStats(ctx, msg); err != nil {
		t.Fatalf("Issue 16: legitimate authority must succeed, got %v", err)
	}
	m, _ := k.GetModel(ctx, "issue16-good-auth")
	if m.WorkerCount != 5 {
		t.Fatalf("Issue 16: WorkerCount must be 5 after accepted update, got %d", m.WorkerCount)
	}
}

// ============================================================
// Issue 9 / 16 — ComputeModelId boundary collision.
// ============================================================

// TestKT_Issue9_ComputeModelId_NoBoundaryCollision pins that adjacent
// variable-length fields produce distinct model_ids — pre-fix
// ("ABC", "DE", "FG") and ("AB", "CDE", "FG") collided.
func TestKT_Issue9_ComputeModelId_NoBoundaryCollision(t *testing.T) {
	idA := keeper.ComputeModelId("ABC", "DE", "FG")
	idB := keeper.ComputeModelId("AB", "CDE", "FG")
	if idA == idB {
		t.Fatalf("Issue 9: boundary-shifted tuples must produce different ids, got %s for both", idA)
	}

	// And the (B, C) boundary too.
	idC := keeper.ComputeModelId("AB", "CD", "EFG")
	idD := keeper.ComputeModelId("AB", "CDE", "FG")
	if idC == idD {
		t.Fatalf("Issue 9: (Q, R) boundary collision still possible, got %s for both", idC)
	}

	// Triple-empty input edge.
	if got := keeper.ComputeModelId("", "", ""); got == "" {
		t.Fatal("Issue 9: empty triple should still produce a deterministic non-empty hash")
	}
}

// TestKT_Issue9_ComputeModelId_StableForSameInput is the existing
// determinism property — changing the hash function MUST keep this true.
func TestKT_Issue9_ComputeModelId_StableForSameInput(t *testing.T) {
	id1 := keeper.ComputeModelId("weight-hash", "quant-hash", "runtime-hash")
	id2 := keeper.ComputeModelId("weight-hash", "quant-hash", "runtime-hash")
	if id1 != id2 {
		t.Fatalf("ComputeModelId not deterministic: %s vs %s", id1, id2)
	}
}

// TestKT_Issue9_ComputeModelId_FieldOrderMatters pins that the formula
// commits to the position of each field — swapping (W, Q) ≠ (Q, W).
func TestKT_Issue9_ComputeModelId_FieldOrderMatters(t *testing.T) {
	idAB := keeper.ComputeModelId("a", "b", "c")
	idBA := keeper.ComputeModelId("b", "a", "c")
	if idAB == idBA {
		t.Fatal("Issue 9: field-order swap must produce different ids")
	}
}
