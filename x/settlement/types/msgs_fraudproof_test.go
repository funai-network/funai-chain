package types

// Tests for FraudProof Phase 2 ValidateBasic enforcement.
//
// External review on PR #54 asked for strict 32-byte length checks on the
// SHA-256 hash fields (TaskId, ReceiptResultHash, ReceivedOutputHash) so
// malformed-length tx are rejected at the mempool gate rather than reaching
// the keeper. These tests pin that contract.

import (
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// makeBech32 returns a bech32-encoded address using the SDK's global
// prefix (default "cosmos" in unit tests; production binaries call
// app.SetAddressPrefixes() at startup to flip it to "funai"). Produces a
// syntactically valid msg.Reporter / WorkerAddress regardless of which
// prefix is active.
func makeBech32(t *testing.T, name string) string {
	t.Helper()
	buf := make([]byte, 20)
	copy(buf, name)
	return sdk.AccAddress(buf).String()
}

// validBaseMsg returns a Phase 2 MsgFraudProof with all hash fields at the
// canonical 32-byte length. Tests mutate one field at a time to assert the
// length check fires for that specific field.
func validBaseMsg(t *testing.T) *MsgFraudProof {
	t.Helper()
	h := func(b byte) []byte {
		out := make([]byte, 32)
		for i := range out {
			out[i] = b
		}
		return out
	}
	return &MsgFraudProof{
		Reporter:           makeBech32(t, "vb-reporter"),
		TaskId:             h(0xAA),
		WorkerAddress:      makeBech32(t, "vb-worker"),
		ReceiptResultHash:  h(0xBB),
		WorkerReceiptSig:   []byte("sig-receipt-placeholder"),
		ReceivedOutputHash: h(0xCC),
		WorkerContentSig:   []byte("sig-content-placeholder"),
	}
}

func TestMsgFraudProof_ValidateBasic_AcceptsCanonical(t *testing.T) {
	msg := validBaseMsg(t)
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("canonical Phase 2 message must pass ValidateBasic: %v", err)
	}
}

func TestMsgFraudProof_ValidateBasic_RejectsShortTaskId(t *testing.T) {
	msg := validBaseMsg(t)
	msg.TaskId = []byte("kt-i52-short-task-id")
	err := msg.ValidateBasic()
	if err == nil || !strings.Contains(err.Error(), "task_id must be 32 bytes") {
		t.Fatalf("expected task_id length error, got %v", err)
	}
}

func TestMsgFraudProof_ValidateBasic_RejectsLongTaskId(t *testing.T) {
	msg := validBaseMsg(t)
	long := make([]byte, 64)
	for i := range long {
		long[i] = 0x11
	}
	msg.TaskId = long
	err := msg.ValidateBasic()
	if err == nil || !strings.Contains(err.Error(), "task_id must be 32 bytes") {
		t.Fatalf("expected task_id length error, got %v", err)
	}
}

func TestMsgFraudProof_ValidateBasic_RejectsShortReceiptResultHash(t *testing.T) {
	msg := validBaseMsg(t)
	msg.ReceiptResultHash = []byte("not-32-bytes")
	err := msg.ValidateBasic()
	if err == nil || !strings.Contains(err.Error(), "receipt_result_hash must be 32 bytes") {
		t.Fatalf("expected receipt_result_hash length error, got %v", err)
	}
}

func TestMsgFraudProof_ValidateBasic_RejectsShortReceivedOutputHash(t *testing.T) {
	msg := validBaseMsg(t)
	msg.ReceivedOutputHash = []byte("not-32-bytes")
	err := msg.ValidateBasic()
	if err == nil || !strings.Contains(err.Error(), "received_output_hash must be 32 bytes") {
		t.Fatalf("expected received_output_hash length error, got %v", err)
	}
}

func TestMsgFraudProof_ValidateBasic_RejectsEmptyWorkerReceiptSig(t *testing.T) {
	msg := validBaseMsg(t)
	msg.WorkerReceiptSig = nil
	err := msg.ValidateBasic()
	if err == nil || !strings.Contains(err.Error(), "worker_receipt_sig") {
		t.Fatalf("expected worker_receipt_sig empty error, got %v", err)
	}
}

func TestMsgFraudProof_ValidateBasic_RejectsEmptyWorkerContentSig(t *testing.T) {
	msg := validBaseMsg(t)
	msg.WorkerContentSig = nil
	err := msg.ValidateBasic()
	if err == nil || !strings.Contains(err.Error(), "worker_content_sig") {
		t.Fatalf("expected worker_content_sig empty error, got %v", err)
	}
}
