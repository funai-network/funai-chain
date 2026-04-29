package keeper_test

// Integration tests for KT 30-case Issue 4 — keeper signature-verify paths
// must accept the worker pubkey in any of the three formats observed on a
// real chain:
//
//   raw    — 33 bytes-as-string, what test fixtures historically produced
//   hex    — 66-char lowercase, what `funaid tx worker register --pubkey`
//            stores (scripts/e2e-real-inference.sh:509)
//   base64 — 44-char padded, what Cosmos SDK keyring default outputs
//
// Pre-fix the FraudProof H3 check did `[]byte(pubkeyStr)` directly. With
// hex-stored pubkeys, that converted the printable hex *characters* into
// "raw bytes" — signature verification then ran against random-looking data
// and never matched. Every legitimate fraud report was silently rejected.
//
// Pre-fix the D2 batch verifier-sig path had `len(pubkeyStr) != 33` — hex
// is 66 chars, so every batch entry was silently rejected with a generic
// "unknown pubkey" log line.
//
// Post-fix all four call sites (FraudProof H3, verifyProposerSigOnRoot, D2
// batch entry sig, p2p decodePubkey) route through types.DecodeWorkerPubkey
// which tries base64 → hex → raw in sequence.

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/keeper"
	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

// ============================================================
// FraudProof H3 with hex / base64 / raw pubkey storage.
// ============================================================

func runFraudProofWithPubkeyFormat(t *testing.T, label, pubkeyOverride string) {
	t.Helper()
	k, ctx, _, wk := setupKeeper(t)
	wk.PubkeyOverride = pubkeyOverride

	workerAddr := makeAddr("kt-i4-fraud-worker")
	taskId := []byte("kt-i4-fraud-task-001")

	contentHash, contentSig := signFraudContent(t, []byte("content"))
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i4-rep").String(),
		TaskId:           taskId,
		WorkerAddress:    workerAddr.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("content"),
	}

	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("[%s] FraudProof must succeed with %s-encoded pubkey: %v", label, label, err)
	}
	if !k.HasFraudMark(ctx, taskId) {
		t.Fatalf("[%s] fraud mark must be set after successful FraudProof", label)
	}
}

func TestKT_Issue4_FraudProof_AcceptsRawPubkey(t *testing.T) {
	// "" override → mock returns raw bytes-as-string (the legacy default).
	runFraudProofWithPubkeyFormat(t, "raw", "")
}

func TestKT_Issue4_FraudProof_AcceptsHexPubkey(t *testing.T) {
	// hex is the format the testnet CLI actually stores.
	runFraudProofWithPubkeyFormat(t, "hex", hex.EncodeToString(testProposerKey.PubKey().Bytes()))
}

func TestKT_Issue4_FraudProof_AcceptsBase64Pubkey(t *testing.T) {
	// base64 is the Cosmos SDK keyring default output.
	runFraudProofWithPubkeyFormat(t, "base64", base64.StdEncoding.EncodeToString(testProposerKey.PubKey().Bytes()))
}

// ============================================================
// MsgBatchSettlement proposer-sig with hex / base64 / raw pubkey storage.
// ============================================================

func runBatchSettlementWithPubkeyFormat(t *testing.T, label, pubkeyOverride string) {
	t.Helper()
	k, ctx, _, wk := setupKeeper(t)
	wk.PubkeyOverride = pubkeyOverride
	k.SetCurrentSecondVerificationRate(ctx, 0)

	user := makeAddr("kt-i4-bs-user")
	worker := makeAddr("kt-i4-bs-worker")
	fee := sdk.NewCoin("ufai", math.NewInt(1_000_000))
	_ = k.ProcessDeposit(ctx, user, sdk.NewCoin("ufai", math.NewInt(5_000_000)))

	verifiers := []types.VerifierResult{
		{Address: makeAddr("kt-i4-bs-v1").String(), Pass: true},
		{Address: makeAddr("kt-i4-bs-v2").String(), Pass: true},
		{Address: makeAddr("kt-i4-bs-v3").String(), Pass: true},
	}
	entries := []types.SettlementEntry{
		{
			TaskId: []byte("kt-i4-bs-task-000001"), UserAddress: user.String(),
			WorkerAddress: worker.String(), Fee: fee, ExpireBlock: 10000,
			Status: types.SettlementSuccess, VerifierResults: verifiers,
		},
	}
	msg := makeBatchMsg(t, makeAddr("kt-i4-bs-prop").String(), entries)
	if _, err := k.ProcessBatchSettlement(ctx, msg); err != nil {
		t.Fatalf("[%s] BatchSettlement must succeed with %s-encoded proposer pubkey: %v", label, label, err)
	}
}

func TestKT_Issue4_ProposerSig_AcceptsRawPubkey(t *testing.T) {
	runBatchSettlementWithPubkeyFormat(t, "raw", "")
}

func TestKT_Issue4_ProposerSig_AcceptsHexPubkey(t *testing.T) {
	runBatchSettlementWithPubkeyFormat(t, "hex", hex.EncodeToString(testProposerKey.PubKey().Bytes()))
}

func TestKT_Issue4_ProposerSig_AcceptsBase64Pubkey(t *testing.T) {
	runBatchSettlementWithPubkeyFormat(t, "base64", base64.StdEncoding.EncodeToString(testProposerKey.PubKey().Bytes()))
}

// ============================================================
// MsgSecondVerificationResultBatch (D2) verifier-sig with hex / base64 /
// raw pubkey storage. Each batch entry carries its own verifier signature;
// the keeper looks up the verifier's on-chain pubkey and decodes it.
// ============================================================

func runD2BatchWithPubkeyFormat(t *testing.T, label, pubkeyOverride string) {
	t.Helper()
	k, ctx, _, wk := setupKeeper(t)
	wk.PubkeyOverride = pubkeyOverride

	taskId := []byte("kt-i4-d2-task-000001")
	rawPubkey := testProposerKey.PubKey().Bytes()

	// Same setup as TestProcessSecondVerificationResultBatch_AcceptsValidSigs.
	seedAuditPending(k, ctx, taskId, []string{makeAddr("kt-i4-d2-orig-v1").String()})

	entries := make([]types.SecondVerificationBatchEntry, 3)
	for i := 0; i < 3; i++ {
		entries[i] = types.SecondVerificationBatchEntry{
			TaskId:               taskId,
			SecondVerifier:       makeAddr("kt-i4-d2-v" + string(rune('1'+i))).String(),
			Epoch:                0,
			Pass:                 true,
			LogitsHash:           []byte("logits-hash"),
			VerifiedInputTokens:  10,
			VerifiedOutputTokens: 20,
		}
		// Sign with the raw key; keeper must decode pubkeyOverride (whatever
		// representation) back to those raw bytes for canonical pre-image
		// equality — the whole point of the helper.
		canonical := keeper.SecondVerificationEntrySigBytes(entries[i], rawPubkey)
		msgHash := sha256.Sum256(canonical)
		signerKey := testProposerKey
		sig, err := secp256k1.PrivKey(signerKey).Sign(msgHash[:])
		if err != nil {
			t.Fatalf("sign D2 entry: %v", err)
		}
		entries[i].Signature = sig
	}

	msg := types.NewMsgSecondVerificationResultBatch(makeAddr("kt-i4-d2-prop").String(), entries)
	accepted, rejected := k.ProcessSecondVerificationResultBatch(ctx, msg)
	if accepted != 3 || rejected != 0 {
		t.Fatalf("[%s] expected 3 accepted 0 rejected with %s-encoded pubkey, got %d/%d",
			label, label, accepted, rejected)
	}
}

func TestKT_Issue4_D2BatchSig_AcceptsRawPubkey(t *testing.T) {
	runD2BatchWithPubkeyFormat(t, "raw", "")
}

func TestKT_Issue4_D2BatchSig_AcceptsHexPubkey(t *testing.T) {
	runD2BatchWithPubkeyFormat(t, "hex", hex.EncodeToString(testProposerKey.PubKey().Bytes()))
}

func TestKT_Issue4_D2BatchSig_AcceptsBase64Pubkey(t *testing.T) {
	runD2BatchWithPubkeyFormat(t, "base64", base64.StdEncoding.EncodeToString(testProposerKey.PubKey().Bytes()))
}
