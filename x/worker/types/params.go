package types

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// DefaultExitWaitPeriod is 21 days in blocks (5s block time): 21*24*3600/5 = 362880.
	DefaultExitWaitPeriod int64 = 362880

	// DefaultColdStartFreeBlocks is 3 days in blocks (5s block time): 3*24*3600/5 = 51840.
	DefaultColdStartFreeBlocks int64 = 51840

	// DefaultJail1Duration is 10 minutes in blocks (5s block time): 10*60/5 = 120.
	DefaultJail1Duration int64 = 120

	// DefaultJail2Duration is 1 hour in blocks (5s block time): 3600/5 = 720.
	DefaultJail2Duration int64 = 720

	// DefaultSlashFraudPercent is 5% stake slash on 3rd jail or FraudProof.
	DefaultSlashFraudPercent uint32 = 5

	// DefaultSuccessResetThreshold is 50 consecutive successes to reset jail_count.
	DefaultSuccessResetThreshold uint32 = 50
)

// DefaultMinStake is 10,000 FAI = 10_000_000_000 ufai.
var DefaultMinStake = sdk.NewCoin("ufai", math.NewInt(10_000_000_000))

type Params struct {
	MinStake              sdk.Coin `protobuf:"bytes,1,opt,name=min_stake,proto3" json:"min_stake"`
	ExitWaitPeriod        int64    `protobuf:"varint,2,opt,name=exit_wait_period,proto3" json:"exit_wait_period"`
	ColdStartFreeBlocks   int64    `protobuf:"varint,3,opt,name=cold_start_free_blocks,proto3" json:"cold_start_free_blocks"`
	Jail1Duration         int64    `protobuf:"varint,4,opt,name=jail_1_duration,proto3" json:"jail_1_duration"`
	Jail2Duration         int64    `protobuf:"varint,5,opt,name=jail_2_duration,proto3" json:"jail_2_duration"`
	SlashFraudPercent     uint32   `protobuf:"varint,6,opt,name=slash_fraud_percent,proto3" json:"slash_fraud_percent"`
	SuccessResetThreshold uint32   `protobuf:"varint,7,opt,name=success_reset_threshold,proto3" json:"success_reset_threshold"`
}

func (m *Params) ProtoMessage()  {}
func (m *Params) Reset()         { *m = Params{} }
func (m *Params) String() string { return "worker.Params" }

func DefaultParams() Params {
	return Params{
		MinStake:              DefaultMinStake,
		ExitWaitPeriod:        DefaultExitWaitPeriod,
		ColdStartFreeBlocks:   DefaultColdStartFreeBlocks,
		Jail1Duration:         DefaultJail1Duration,
		Jail2Duration:         DefaultJail2Duration,
		SlashFraudPercent:     DefaultSlashFraudPercent,
		SuccessResetThreshold: DefaultSuccessResetThreshold,
	}
}

func (p Params) Validate() error {
	if !p.MinStake.IsValid() || p.MinStake.IsZero() {
		return ErrInsufficientStake
	}
	if p.ExitWaitPeriod <= 0 {
		return ErrExitWaitPeriod
	}
	if p.ColdStartFreeBlocks < 0 {
		return ErrExitWaitPeriod
	}
	if p.Jail1Duration <= 0 || p.Jail2Duration <= 0 {
		return ErrExitWaitPeriod
	}
	if p.SlashFraudPercent == 0 || p.SlashFraudPercent > 100 {
		return ErrInsufficientStake
	}
	if p.SuccessResetThreshold == 0 {
		return ErrInsufficientStake
	}
	return nil
}
