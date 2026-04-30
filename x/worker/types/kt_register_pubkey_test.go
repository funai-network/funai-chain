package types_test

// Tests for KT 30-case Issue 14 — MsgRegisterWorker pubkey strict validation.
//
// Pre-fix MsgRegisterWorker.ValidateBasic only checked `msg.Pubkey != ""`.
// A worker could register with `Pubkey="garbage"`; downstream sig-verify
// paths (FraudProof H3, D2 audit batch, Proposer signature) accept the
// registration as a valid participant but fail every signed-message check,
// allowing the worker to occupy committee slots while contributing zero —
// a low-cost grief attack.
//
// Post-fix ValidateBasic routes through types.DecodeWorkerPubkey, which
// accepts any of the three observed formats (raw 33 bytes-as-string, hex,
// or base64) and rejects everything else.

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/worker/types"
)

// makeRegisterMsg builds a MsgRegisterWorker with a given pubkey string;
// other fields are minimal-valid so we isolate pubkey validation.
func makeRegisterMsg(pubkey string) *types.MsgRegisterWorker {
	creator := sdk.AccAddress([]byte("kt-i14-creator______")).String()
	return types.NewMsgRegisterWorker(
		creator,
		pubkey,
		[]string{"some-model"},
		"/ip4/127.0.0.1/tcp/15000",
		"Tesla-T4",
		15, 1,
		"op-id",
		1,
	)
}

func TestKT_Issue14_RegisterWorker_RejectsGarbagePubkey(t *testing.T) {
	cases := []string{
		"garbage",
		"abc",
		"this-is-not-a-pubkey",
		"x",
	}
	for _, c := range cases {
		msg := makeRegisterMsg(c)
		if err := msg.ValidateBasic(); err == nil {
			t.Fatalf("Issue 14: ValidateBasic must reject pubkey %q, got nil error", c)
		}
	}
}

func TestKT_Issue14_RegisterWorker_AcceptsRawBytesAsString(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	rawPubkey := string(priv.PubKey().Bytes())
	msg := makeRegisterMsg(rawPubkey)
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("Issue 14: raw-bytes-as-string must pass, got %v", err)
	}
}

func TestKT_Issue14_RegisterWorker_AcceptsHex(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	hexPubkey := hex.EncodeToString(priv.PubKey().Bytes())
	msg := makeRegisterMsg(hexPubkey)
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("Issue 14: hex-encoded pubkey must pass (the testnet CLI format), got %v", err)
	}
}

func TestKT_Issue14_RegisterWorker_AcceptsBase64(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	b64Pubkey := base64.StdEncoding.EncodeToString(priv.PubKey().Bytes())
	msg := makeRegisterMsg(b64Pubkey)
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("Issue 14: base64-encoded pubkey (Cosmos SDK keyring default) must pass, got %v", err)
	}
}

func TestKT_Issue14_RegisterWorker_RejectsEmptyPubkey(t *testing.T) {
	// Pre-existing behavior — the empty check fires before the format check.
	msg := makeRegisterMsg("")
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("empty pubkey must still be rejected")
	}
}

func TestKT_Issue14_RegisterWorker_RejectsWrongLengthHex(t *testing.T) {
	// Valid hex chars but wrong byte length (32 bytes ≠ 33 compressed).
	msg := makeRegisterMsg(hex.EncodeToString(make([]byte, 32)))
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("Issue 14: 32-byte (wrong length) hex pubkey must be rejected")
	}
}
