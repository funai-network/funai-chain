package types

import "encoding/binary"

const (
	ModuleName        = "settlement"
	StoreKey          = ModuleName
	RouterKey         = ModuleName
	QuerierRoute      = ModuleName
	ModuleAccountName = ModuleName
	DefaultDenom      = "ufai"
)

var (
	InferenceAccountKeyPrefix       = []byte{0x01}
	SettledTaskKeyPrefix            = []byte{0x02}
	FraudMarkKeyPrefix              = []byte{0x03}
	AuditRecordKeyPrefix            = []byte{0x04}
	ParamsKey                       = []byte{0x05}
	BatchRecordKeyPrefix            = []byte{0x06}
	BatchCounterKey                 = []byte{0x07}
	AuditPendingKeyPrefix           = []byte{0x08}
	ReauditPendingKeyPrefix         = []byte{0x09}
	EpochStatsKeyPrefix             = []byte{0x0A}
	AuditRateKey                    = []byte{0x0B}
	ReauditRateKey                  = []byte{0x0C}
	AuditPendingTimeoutKeyPrefix    = []byte{0x0D} // height-indexed for efficient timeout lookup
	ReauditPendingTimeoutKeyPrefix  = []byte{0x0E}
	WorkerSnapshotKeyPrefix         = []byte{0x0F} // P1-8: per-worker epoch snapshot
	WorkerEpochContribKeyPrefix     = []byte{0x10} // P1-8: per-worker epoch contribution
	VerifierEpochCountKeyPrefix     = []byte{0x11} // P1-9: per-worker verification count in epoch
	AuditorEpochCountKeyPrefix      = []byte{0x12} // P1-9: per-worker audit count in epoch
	BlockSignerCountKeyPrefix       = []byte{0x13} // P1-10: per-validator block signing count in epoch
	DishonestCountKeyPrefix         = []byte{0x14} // S9: per-worker dishonest token count
	FrozenBalanceKeyPrefix          = []byte{0x15} // S9: per-task frozen max_fee
	FrozenTaskIndexKeyPrefix        = []byte{0x16} // S9: expireBlock→taskId index for timeout scan
	TokenMismatchKeyPrefix          = []byte{0x17} // S9: Worker-Verifier pair mismatch tracking
)

func InferenceAccountKey(userAddr []byte) []byte {
	return append(InferenceAccountKeyPrefix, userAddr...)
}

func SettledTaskKey(taskID []byte) []byte {
	return append(SettledTaskKeyPrefix, taskID...)
}

func FraudMarkKey(taskID []byte) []byte {
	return append(FraudMarkKeyPrefix, taskID...)
}

func AuditRecordKey(taskID []byte) []byte {
	return append(AuditRecordKeyPrefix, taskID...)
}

func BatchRecordKey(batchId uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, batchId)
	return append(BatchRecordKeyPrefix, bz...)
}

func AuditPendingKey(taskID []byte) []byte {
	return append(AuditPendingKeyPrefix, taskID...)
}

func ReauditPendingKey(taskID []byte) []byte {
	return append(ReauditPendingKeyPrefix, taskID...)
}

func EpochStatsKey(epoch int64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(epoch))
	return append(EpochStatsKeyPrefix, bz...)
}

// WorkerSnapshotKey returns the key for a worker's epoch snapshot.
func WorkerSnapshotKey(workerAddr []byte) []byte {
	return append(WorkerSnapshotKeyPrefix, workerAddr...)
}

// WorkerEpochContribKey returns the key for a worker's current epoch contribution.
func WorkerEpochContribKey(workerAddr []byte) []byte {
	return append(WorkerEpochContribKeyPrefix, workerAddr...)
}

// VerifierEpochCountKey returns the key for a verifier's epoch verification count.
func VerifierEpochCountKey(workerAddr []byte) []byte {
	return append(VerifierEpochCountKeyPrefix, workerAddr...)
}

// AuditorEpochCountKey returns the key for an auditor's epoch audit count.
func AuditorEpochCountKey(workerAddr []byte) []byte {
	return append(AuditorEpochCountKeyPrefix, workerAddr...)
}

// BlockSignerCountKey returns key for a validator's block signing count.
func BlockSignerCountKey(validatorAddr string) []byte {
	return append(BlockSignerCountKeyPrefix, []byte(validatorAddr)...)
}

// DishonestCountKey returns the key for a worker's dishonest token reporting count (S9).
func DishonestCountKey(workerAddr []byte) []byte {
	return append(DishonestCountKeyPrefix, workerAddr...)
}

// FrozenBalanceKey returns the key for a task's frozen max_fee (S9).
func FrozenBalanceKey(taskID []byte) []byte {
	return append(FrozenBalanceKeyPrefix, taskID...)
}

// FrozenTaskIndexKey stores expireBlock + taskId for efficient timeout scanning.
// Format: prefix + expireBlock(8 bytes BE) + taskId
func FrozenTaskIndexKey(expireBlock int64, taskID []byte) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(expireBlock))
	key := append(FrozenTaskIndexKeyPrefix, bz...)
	return append(key, taskID...)
}

// TokenMismatchKey returns key for a Worker-Verifier pair mismatch record.
func TokenMismatchKey(workerAddr, verifierAddr string) []byte {
	key := append(TokenMismatchKeyPrefix, []byte(workerAddr)...)
	key = append(key, byte('|'))
	return append(key, []byte(verifierAddr)...)
}

// TokenMismatchPrefixForWorker returns prefix for scanning all pairs of a worker.
func TokenMismatchPrefixForWorker(workerAddr string) []byte {
	key := append(TokenMismatchKeyPrefix, []byte(workerAddr)...)
	return append(key, byte('|'))
}

// AuditPendingTimeoutKey returns key: prefix + height(8 bytes) + taskID.
// Enables efficient range scan for timed-out tasks by height.
func AuditPendingTimeoutKey(height int64, taskID []byte) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(height))
	key := append(AuditPendingTimeoutKeyPrefix, bz...)
	return append(key, taskID...)
}

func ReauditPendingTimeoutKey(height int64, taskID []byte) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(height))
	key := append(ReauditPendingTimeoutKeyPrefix, bz...)
	return append(key, taskID...)
}

// AuditPendingTimeoutPrefixUpTo returns prefix for scanning all timeout keys up to a given height (inclusive).
func AuditPendingTimeoutPrefixUpTo(maxHeight int64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(maxHeight+1))
	return append(AuditPendingTimeoutKeyPrefix, bz...)
}

func ReauditPendingTimeoutPrefixUpTo(maxHeight int64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(maxHeight+1))
	return append(ReauditPendingTimeoutKeyPrefix, bz...)
}
