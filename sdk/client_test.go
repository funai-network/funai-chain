package sdk

import (
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"

	p2ptypes "github.com/funai-wiki/funai-chain/p2p/types"
)

// ── Worker-sign → SDK-verify round-trip ──────────────────────────────────────
//
// Regression tests for the SDK receipt-signature verification path.
//
// Previously `verifyWorkerReceiptSig` had an extra sha256.Sum256(SignBytes())
// step on top of cometbft's internal sha256, producing a 3-layer digest that
// never matched Worker's 2-layer Sign(SignBytes()) output. Result: the SDK
// silently rejected every real Worker receipt ("signature invalid, ignoring")
// and the M7 FRAUD-DETECTED branch was unreachable. These tests make sure
// future refactors cannot reintroduce the same mismatch.

// signReceiptLikeWorker mirrors p2p/worker.signReceipt: pass receipt.SignBytes()
// directly to cometbft secp256k1 Sign, which internally sha256's the input.
// Keep this helper identical to the real Worker signer — if it drifts, the
// test stops being a regression for the real production path.
func signReceiptLikeWorker(t *testing.T, r *p2ptypes.InferReceipt, priv secp256k1.PrivKey) {
	t.Helper()
	// WorkerPubkey must be populated BEFORE signing because
	// InferReceipt.SignBytes writes it into the digest. This matches the
	// order in p2p/worker.HandleTask which sets WorkerPubkey at receipt
	// creation and only then calls signReceipt.
	r.WorkerPubkey = priv.PubKey().Bytes()
	sig, err := priv.Sign(r.SignBytes())
	if err != nil {
		t.Fatalf("worker sign: %v", err)
	}
	r.WorkerSig = sig
}

func fixtureReceipt(inferenceLatencyMs uint32) *p2ptypes.InferReceipt {
	return &p2ptypes.InferReceipt{
		TaskId:             []byte("task-id-sdk-rt-1"),
		WorkerLogits:       [5]float32{1.0, 2.0, 3.0, 4.0, 5.0},
		ResultHash:         []byte("result-hash-0000"),
		FinalSeed:          []byte("final-seed-00"),
		SampledTokens:      [5]uint32{10, 20, 30, 40, 50},
		InputTokenCount:    42,
		OutputTokenCount:   7,
		InferenceLatencyMs: inferenceLatencyMs,
	}
}

func TestVerifyWorkerReceiptSig_HappyPath(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	r := fixtureReceipt(250)
	signReceiptLikeWorker(t, r, priv)

	if !verifyWorkerReceiptSig(r) {
		t.Fatal("SDK must accept a receipt signed the same way p2p/worker.signReceipt does")
	}
}

func TestVerifyWorkerReceiptSig_RejectsTamperedResultHash(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	r := fixtureReceipt(250)
	signReceiptLikeWorker(t, r, priv)

	r.ResultHash = []byte("result-hash-EVIL")

	if verifyWorkerReceiptSig(r) {
		t.Fatal("tampered result_hash must invalidate Worker signature (M7 fraud detection relies on this)")
	}
}

func TestVerifyWorkerReceiptSig_RejectsTamperedInferenceLatencyMs(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	r := fixtureReceipt(250)
	signReceiptLikeWorker(t, r, priv)

	r.InferenceLatencyMs = 10 // attacker tries to make this Worker look 25x faster

	if verifyWorkerReceiptSig(r) {
		t.Fatal("tampered InferenceLatencyMs must invalidate Worker signature (Audit KT §5)")
	}
}

func TestVerifyWorkerReceiptSig_RejectsWrongPubkey(t *testing.T) {
	signer := secp256k1.GenPrivKey()
	attackerPubkey := secp256k1.GenPrivKey().PubKey().Bytes()

	r := fixtureReceipt(250)
	signReceiptLikeWorker(t, r, signer)

	r.WorkerPubkey = attackerPubkey

	if verifyWorkerReceiptSig(r) {
		t.Fatal("receipt presented with a different pubkey than the signer must be rejected")
	}
}

func TestVerifyWorkerReceiptSig_RejectsMalformedInputs(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	good := fixtureReceipt(250)
	signReceiptLikeWorker(t, good, priv)

	t.Run("nil receipt", func(t *testing.T) {
		if verifyWorkerReceiptSig(nil) {
			t.Fatal("nil receipt must be rejected")
		}
	})

	t.Run("empty sig", func(t *testing.T) {
		r := *good
		r.WorkerSig = nil
		if verifyWorkerReceiptSig(&r) {
			t.Fatal("empty sig must be rejected")
		}
	})

	t.Run("non-33-byte pubkey", func(t *testing.T) {
		r := *good
		r.WorkerPubkey = []byte{0x01, 0x02, 0x03}
		if verifyWorkerReceiptSig(&r) {
			t.Fatal("short pubkey must be rejected (expected 33-byte compressed secp256k1)")
		}
	})
}
