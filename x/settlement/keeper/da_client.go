package keeper

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

// ComputeEntryHash hashes a single SettlementEntry for Merkle tree construction.
// Covers all fields (not just TaskId) to ensure data integrity.
func ComputeEntryHash(e types.SettlementEntry) []byte {
	bz, _ := json.Marshal(e)
	h := sha256.Sum256(bz)
	return h[:]
}

// ComputeMerkleRoot computes a binary Merkle tree root from settlement entries.
// Each leaf is SHA256(JSON(entry)). If odd number of leaves, the last is duplicated.
func ComputeMerkleRoot(entries []types.SettlementEntry) []byte {
	if len(entries) == 0 {
		return nil
	}

	hashes := make([][]byte, len(entries))
	for i, e := range entries {
		hashes[i] = ComputeEntryHash(e)
	}

	for len(hashes) > 1 {
		var next [][]byte
		for i := 0; i < len(hashes); i += 2 {
			if i+1 < len(hashes) {
				next = append(next, hashPair(hashes[i], hashes[i+1]))
			} else {
				next = append(next, hashPair(hashes[i], hashes[i]))
			}
		}
		hashes = next
	}

	return hashes[0]
}

func hashPair(a, b []byte) []byte {
	h := sha256.New()
	h.Write(a)
	h.Write(b)
	sum := h.Sum(nil)
	return sum
}

// VerifyMerkleRoot verifies that the merkle root matches the entries.
func VerifyMerkleRoot(merkleRoot []byte, entries []types.SettlementEntry) bool {
	if len(entries) == 0 {
		return false
	}
	computed := ComputeMerkleRoot(entries)
	if len(merkleRoot) != len(computed) {
		return false
	}
	for i := range merkleRoot {
		if merkleRoot[i] != computed[i] {
			return false
		}
	}
	return true
}

// ComputeReserveEntryHash hashes a single ReserveEntry for merkle tree construction.
func ComputeReserveEntryHash(e types.ReserveEntry) []byte {
	bz, _ := json.Marshal(e)
	h := sha256.Sum256(bz)
	return h[:]
}

// ComputeReserveMerkleRoot mirrors ComputeMerkleRoot but for MsgBatchReserve
// entries. The Proposer's signature covers the resulting root, so any tamper
// of an entry post-sign invalidates the batch (same security model as
// BatchSettlement).
func ComputeReserveMerkleRoot(entries []types.ReserveEntry) []byte {
	if len(entries) == 0 {
		return nil
	}
	hashes := make([][]byte, len(entries))
	for i, e := range entries {
		hashes[i] = ComputeReserveEntryHash(e)
	}
	for len(hashes) > 1 {
		var next [][]byte
		for i := 0; i < len(hashes); i += 2 {
			if i+1 < len(hashes) {
				next = append(next, hashPair(hashes[i], hashes[i+1]))
			} else {
				next = append(next, hashPair(hashes[i], hashes[i]))
			}
		}
		hashes = next
	}
	return hashes[0]
}

// VerifyReserveMerkleRoot mirrors VerifyMerkleRoot for ReserveEntry.
func VerifyReserveMerkleRoot(merkleRoot []byte, entries []types.ReserveEntry) bool {
	if len(entries) == 0 {
		return false
	}
	computed := ComputeReserveMerkleRoot(entries)
	if len(merkleRoot) != len(computed) {
		return false
	}
	for i := range merkleRoot {
		if merkleRoot[i] != computed[i] {
			return false
		}
	}
	return true
}
