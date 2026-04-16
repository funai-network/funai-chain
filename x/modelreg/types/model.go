package types

import (
	"fmt"
	"regexp"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var aliasRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$`)

// ValidateAlias checks that an alias is 3-64 chars, lowercase alphanumeric + hyphen.
func ValidateAlias(alias string) error {
	if len(alias) < 3 || len(alias) > 64 {
		return fmt.Errorf("alias must be 3-64 characters, got %d", len(alias))
	}
	if !aliasRegex.MatchString(alias) {
		return fmt.Errorf("alias must contain only lowercase letters, digits, and hyphens (cannot start/end with hyphen)")
	}
	return nil
}

type ModelStatus uint32

const (
	ModelStatusProposed ModelStatus = 0
	ModelStatusActive   ModelStatus = 1
)

func (s ModelStatus) String() string {
	switch s {
	case ModelStatusProposed:
		return "MODEL_PROPOSED"
	case ModelStatusActive:
		return "MODEL_ACTIVE"
	default:
		return "UNKNOWN"
	}
}

// Model represents a registered AI model in the decentralized inference network.
// ModelId = SHA256(weight_hash || quant_config_hash || runtime_image_hash).
type Model struct {
	ModelId             string      `protobuf:"bytes,1,opt,name=model_id,proto3" json:"model_id"`
	Name                string      `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Alias               string      `protobuf:"bytes,21,opt,name=alias,proto3" json:"alias"`
	Epsilon             uint32      `protobuf:"varint,3,opt,name=epsilon,proto3" json:"epsilon"`
	Status              ModelStatus `protobuf:"varint,4,opt,name=status,proto3" json:"status"`
	ProposerAddress     string      `protobuf:"bytes,5,opt,name=proposer_address,proto3" json:"proposer_address"`
	WeightHash          string      `protobuf:"bytes,6,opt,name=weight_hash,proto3" json:"weight_hash"`
	QuantConfigHash     string      `protobuf:"bytes,7,opt,name=quant_config_hash,proto3" json:"quant_config_hash"`
	RuntimeImageHash    string      `protobuf:"bytes,8,opt,name=runtime_image_hash,proto3" json:"runtime_image_hash"`
	InstalledStakeRatio float64     `protobuf:"fixed64,9,opt,name=installed_stake_ratio,proto3" json:"installed_stake_ratio"`
	WorkerCount         uint32      `protobuf:"varint,10,opt,name=worker_count,proto3" json:"worker_count"`
	OperatorCount       uint32      `protobuf:"varint,11,opt,name=operator_count,proto3" json:"operator_count"`
	SuggestedPrice      sdk.Coin    `protobuf:"bytes,12,opt,name=suggested_price,proto3" json:"suggested_price"`
	ActivatedAt         int64       `protobuf:"varint,13,opt,name=activated_at,proto3" json:"activated_at"`
	CreatedAt           int64       `protobuf:"varint,14,opt,name=created_at,proto3" json:"created_at"`

	// M13: model-level runtime statistics (updated per epoch)
	ActiveWorkers  uint32 `protobuf:"varint,15,opt,name=active_workers,proto3" json:"active_workers"`
	TpsLastEpoch   uint32 `protobuf:"varint,16,opt,name=tps_last_epoch,proto3" json:"tps_last_epoch"`
	AvgFee         uint64 `protobuf:"varint,17,opt,name=avg_fee,proto3" json:"avg_fee"`
	AvgLatencyMs   uint64 `protobuf:"varint,18,opt,name=avg_latency_ms,proto3" json:"avg_latency_ms"`
	TotalTasks24h  uint64 `protobuf:"varint,19,opt,name=total_tasks_24h,proto3" json:"total_tasks_24h"`
	LastStatsEpoch int64  `protobuf:"varint,20,opt,name=last_stats_epoch,proto3" json:"last_stats_epoch"`
}

func (m *Model) ProtoMessage()  {}
func (m *Model) Reset()         { *m = Model{} }
func (m *Model) String() string { return fmt.Sprintf("Model{%s,%s}", m.ModelId, m.Name) }

// IsActivated checks the V5.1 activation thresholds:
// installed_stake_ratio >= 2/3 AND workers >= 4 AND operators >= 4.
func (m Model) IsActivated() bool {
	return m.InstalledStakeRatio >= 2.0/3.0 &&
		m.WorkerCount >= 4 &&
		m.OperatorCount >= 4
}

// CanServe checks the service threshold.
// Audit KT §6: down-line if worker count OR stake ratio drops below threshold.
// serviceStakeRatio is the minimum InstalledStakeRatio (default 2/3 for anti-sybil).
func (m Model) CanServe(minWorkerCount uint32, serviceStakeRatio float64) bool {
	return m.InstalledStakeRatio >= serviceStakeRatio && m.WorkerCount >= minWorkerCount
}
