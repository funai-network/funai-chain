package types

import (
	"encoding/base64"
	"encoding/hex"
)

// CompressedSecp256k1PubkeyLen is the byte length of a compressed secp256k1
// public key. Cosmos SDK uses this format throughout its key infrastructure.
const CompressedSecp256k1PubkeyLen = 33

// DecodeWorkerPubkey accepts the worker pubkey in any of the three formats
// observed across the codebase and returns the raw 33-byte compressed
// secp256k1 representation. Returns nil on unrecognized / wrong-length input.
//
// The three formats:
//
//   - hex      — what `funaid tx worker register --pubkey "<hex>"` stores
//                (see scripts/e2e-real-inference.sh:509 — the actual testnet
//                CLI path; results in a 66-character lowercase hex string)
//   - base64   — Cosmos SDK keyring default; what `keys show <name> --output json`
//                produces; results in a 44-character padded base64 string
//   - raw      — what test fixtures and some legacy paths produce: the 33
//                bytes-as-string ("\x02\xab..." rather than a printable form)
//
// Centralizing the decode here closes KT 30-case Issue 4: pre-fix the
// FraudProof H3 check at x/settlement/keeper/keeper.go did `[]byte(pubkeyStr)`
// directly, treating a hex-stored pubkey's printable characters as if they
// were raw bytes — signature verification therefore never matched in
// production, silently breaking every legitimate fraud report. The D2 batch
// verifier-sig path had the same inversion (`len != 33` rejected hex's 66
// chars). verifyProposerSigOnRoot had a hex+raw fallback so it worked, but
// did not handle the base64 form.
//
// Order matters: we try base64 FIRST because:
//   - Cosmos SDK's default keyring output is base64, so it's the most common
//     format on a real-world chain
//   - base64 imposes specific padding ('=' suffix) and a restricted alphabet
//     that is unlikely to coincidentally decode to 33 bytes from a non-base64
//     input — it's the most distinguishable
//
// Then hex (66-char lowercase, also distinguishable from raw by length); raw
// is the fallback for any 33-byte string-as-bytes input.
func DecodeWorkerPubkey(s string) []byte {
	if s == "" {
		return nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == CompressedSecp256k1PubkeyLen {
		return b
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) == CompressedSecp256k1PubkeyLen {
		return b
	}
	if b := []byte(s); len(b) == CompressedSecp256k1PubkeyLen {
		return b
	}
	return nil
}
