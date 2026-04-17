package types

import "fmt"

const (
	DefaultSignatureExpireMax     int64  = 17280
	DefaultExecutorFeeRatio       uint32 = 850  // 85.0% — Audit KT §8: worker_reward
	DefaultVerifierFeeRatio       uint32 = 120  // 12.0% — Audit KT §8: verifier_reward (3 verifiers, ~4% each)
	DefaultAuditFundRatio         uint32 = 30   // 3.0%  — Audit KT §8: audit_fund (independent)
	DefaultFailSettlementFeeRatio uint32 = 150 // 15% — matches verifier 12% + audit fund 3% (= non-worker share of success fee)
	DefaultAuditVerifierCount     uint32 = 3
	DefaultAuditMatchThreshold    uint32 = 2
	DefaultTaskCleanupBuffer      int64  = 1000
	DefaultLogitsSamplePositions  uint32 = 5
	DefaultLogitsMatchRequired    uint32 = 4

	// V5.2: per-task VRF audit
	DefaultAuditBaseRate   uint32 = 100 // 10.0% (per-mille)
	DefaultAuditRateMin    uint32 = 50  // 5.0%
	DefaultAuditRateMax    uint32 = 300 // 30.0%
	DefaultAuditTimeout    int64  = 8640  // 12 hours in blocks (12*3600/5)
	DefaultReauditBaseRate uint32 = 10  // 1.0%
	DefaultReauditRateMin  uint32 = 5   // 0.5%
	DefaultReauditRateMax  uint32 = 50  // 5.0%
	DefaultReauditTimeout  int64  = 17280 // 24 hours in blocks (24*3600/5)

	// S9: per-token billing params
	DefaultTokenCountTolerance        uint32 = 2     // absolute tolerance for token count mismatch (S9 §3.4)
	DefaultDishonestJailThreshold     uint32 = 3     // jail after 3 dishonest reports (S9 §3.4 Case B)
	DefaultPerTokenBillingEnabled     bool   = false // governance toggle — requires vote to enable
	DefaultTokenCountTolerancePct     uint32 = 2     // percentage tolerance for token count mismatch (S9 §3.4)
	DefaultTokenMismatchAuditWeight   uint32 = 20    // max audit rate boost from pair tracking (S9 §5.2.4)
	DefaultTokenMismatchLookback      uint32 = 100   // sliding window size for pair stats (S9 §5.2.3)
	DefaultTokenMismatchDeviationPct  uint32 = 20    // deviation threshold to count as mismatch
	DefaultTokenMismatchPairMinSamples uint32 = 5    // min samples before pair is considered
)

type Params struct {
	SignatureExpireMax     int64  `protobuf:"varint,1,opt,name=signature_expire_max,proto3" json:"signature_expire_max"`
	ExecutorFeeRatio      uint32 `protobuf:"varint,2,opt,name=executor_fee_ratio,proto3" json:"executor_fee_ratio"`
	VerifierFeeRatio      uint32 `protobuf:"varint,3,opt,name=verifier_fee_ratio,proto3" json:"verifier_fee_ratio"`
	AuditFundRatio        uint32 `protobuf:"varint,4,opt,name=audit_fund_ratio,proto3" json:"audit_fund_ratio"`
	FailSettlementFeeRatio uint32 `protobuf:"varint,5,opt,name=fail_settlement_fee_ratio,proto3" json:"fail_settlement_fee_ratio"`
	AuditVerifierCount    uint32 `protobuf:"varint,6,opt,name=audit_verifier_count,proto3" json:"audit_verifier_count"`
	AuditMatchThreshold   uint32 `protobuf:"varint,7,opt,name=audit_match_threshold,proto3" json:"audit_match_threshold"`
	TaskCleanupBuffer     int64  `protobuf:"varint,8,opt,name=task_cleanup_buffer,proto3" json:"task_cleanup_buffer"`
	LogitsSamplePositions uint32 `protobuf:"varint,9,opt,name=logits_sample_positions,proto3" json:"logits_sample_positions"`
	LogitsMatchRequired   uint32 `protobuf:"varint,10,opt,name=logits_match_required,proto3" json:"logits_match_required"`

	// V5.2: per-task VRF audit rate (per-mille, e.g. 100 = 10%)
	AuditBaseRate   uint32 `protobuf:"varint,11,opt,name=audit_base_rate,proto3" json:"audit_base_rate"`
	AuditRateMin    uint32 `protobuf:"varint,12,opt,name=audit_rate_min,proto3" json:"audit_rate_min"`
	AuditRateMax    uint32 `protobuf:"varint,13,opt,name=audit_rate_max,proto3" json:"audit_rate_max"`
	AuditTimeout    int64  `protobuf:"varint,14,opt,name=audit_timeout,proto3" json:"audit_timeout"`

	// V5.2: reaudit rate (per-mille, e.g. 10 = 1%)
	ReauditBaseRate uint32 `protobuf:"varint,15,opt,name=reaudit_base_rate,proto3" json:"reaudit_base_rate"`
	ReauditRateMin  uint32 `protobuf:"varint,16,opt,name=reaudit_rate_min,proto3" json:"reaudit_rate_min"`
	ReauditRateMax  uint32 `protobuf:"varint,17,opt,name=reaudit_rate_max,proto3" json:"reaudit_rate_max"`
	ReauditTimeout  int64  `protobuf:"varint,18,opt,name=reaudit_timeout,proto3" json:"reaudit_timeout"`

	// S9: per-token billing params
	TokenCountTolerance        uint32 `protobuf:"varint,20,opt,name=token_count_tolerance,proto3" json:"token_count_tolerance"`
	DishonestJailThreshold     uint32 `protobuf:"varint,21,opt,name=dishonest_jail_threshold,proto3" json:"dishonest_jail_threshold"`
	PerTokenBillingEnabled     bool   `protobuf:"varint,22,opt,name=per_token_billing_enabled,proto3" json:"per_token_billing_enabled"`
	TokenCountTolerancePct     uint32 `protobuf:"varint,23,opt,name=token_count_tolerance_pct,proto3" json:"token_count_tolerance_pct"`
	TokenMismatchAuditWeight   uint32 `protobuf:"varint,24,opt,name=token_mismatch_audit_weight,proto3" json:"token_mismatch_audit_weight"`
	TokenMismatchLookback      uint32 `protobuf:"varint,25,opt,name=token_mismatch_lookback,proto3" json:"token_mismatch_lookback"`
	TokenMismatchDeviationPct  uint32 `protobuf:"varint,26,opt,name=token_mismatch_deviation_pct,proto3" json:"token_mismatch_deviation_pct"`
	TokenMismatchPairMinSamples uint32 `protobuf:"varint,27,opt,name=token_mismatch_pair_min_samples,proto3" json:"token_mismatch_pair_min_samples"`
}

func (m *Params) ProtoMessage()  {}
func (m *Params) Reset()         { *m = Params{} }
func (m *Params) String() string { return "settlement.Params" }

func DefaultParams() Params {
	return Params{
		SignatureExpireMax:      DefaultSignatureExpireMax,
		ExecutorFeeRatio:       DefaultExecutorFeeRatio,
		VerifierFeeRatio:       DefaultVerifierFeeRatio,
		AuditFundRatio:         DefaultAuditFundRatio,
		FailSettlementFeeRatio: DefaultFailSettlementFeeRatio,
		AuditVerifierCount:     DefaultAuditVerifierCount,
		AuditMatchThreshold:    DefaultAuditMatchThreshold,
		TaskCleanupBuffer:      DefaultTaskCleanupBuffer,
		LogitsSamplePositions:  DefaultLogitsSamplePositions,
		LogitsMatchRequired:    DefaultLogitsMatchRequired,
		AuditBaseRate:          DefaultAuditBaseRate,
		AuditRateMin:           DefaultAuditRateMin,
		AuditRateMax:           DefaultAuditRateMax,
		AuditTimeout:           DefaultAuditTimeout,
		ReauditBaseRate:        DefaultReauditBaseRate,
		ReauditRateMin:         DefaultReauditRateMin,
		ReauditRateMax:         DefaultReauditRateMax,
		ReauditTimeout:         DefaultReauditTimeout,
		TokenCountTolerance:         DefaultTokenCountTolerance,
		DishonestJailThreshold:      DefaultDishonestJailThreshold,
		PerTokenBillingEnabled:      DefaultPerTokenBillingEnabled,
		TokenCountTolerancePct:      DefaultTokenCountTolerancePct,
		TokenMismatchAuditWeight:    DefaultTokenMismatchAuditWeight,
		TokenMismatchLookback:       DefaultTokenMismatchLookback,
		TokenMismatchDeviationPct:   DefaultTokenMismatchDeviationPct,
		TokenMismatchPairMinSamples: DefaultTokenMismatchPairMinSamples,
	}
}

func (p Params) Validate() error {
	if p.SignatureExpireMax <= 0 {
		return fmt.Errorf("signature_expire_max must be positive, got %d", p.SignatureExpireMax)
	}
	if p.ExecutorFeeRatio == 0 {
		return fmt.Errorf("executor_fee_ratio must be positive")
	}
	if p.VerifierFeeRatio == 0 {
		return fmt.Errorf("verifier_fee_ratio must be positive")
	}
	if p.AuditFundRatio == 0 {
		return fmt.Errorf("audit_fund_ratio must be positive")
	}
	feeRatioSum := p.ExecutorFeeRatio + p.VerifierFeeRatio + p.AuditFundRatio
	if feeRatioSum != 1000 {
		return fmt.Errorf("fee ratios (executor+verifier+audit) must sum to 1000 (per-mille), got %d", feeRatioSum)
	}
	if p.FailSettlementFeeRatio == 0 || p.FailSettlementFeeRatio > 1000 {
		return fmt.Errorf("fail_settlement_fee_ratio must be between 1 and 1000, got %d", p.FailSettlementFeeRatio)
	}
	if p.AuditVerifierCount == 0 {
		return fmt.Errorf("audit_verifier_count must be positive")
	}
	if p.AuditMatchThreshold == 0 || p.AuditMatchThreshold > p.AuditVerifierCount {
		return fmt.Errorf("audit_match_threshold must be between 1 and audit_verifier_count")
	}
	if p.TaskCleanupBuffer < 0 {
		return fmt.Errorf("task_cleanup_buffer cannot be negative, got %d", p.TaskCleanupBuffer)
	}
	if p.LogitsSamplePositions == 0 {
		return fmt.Errorf("logits_sample_positions must be positive")
	}
	if p.LogitsMatchRequired == 0 || p.LogitsMatchRequired > p.LogitsSamplePositions {
		return fmt.Errorf("logits_match_required must be between 1 and logits_sample_positions")
	}
	if p.AuditBaseRate == 0 || p.AuditBaseRate > 1000 {
		return fmt.Errorf("audit_base_rate must be between 1 and 1000, got %d", p.AuditBaseRate)
	}
	if p.AuditRateMin > p.AuditRateMax {
		return fmt.Errorf("audit_rate_min (%d) must not exceed audit_rate_max (%d)", p.AuditRateMin, p.AuditRateMax)
	}
	if p.AuditTimeout <= 0 {
		return fmt.Errorf("audit_timeout must be positive, got %d", p.AuditTimeout)
	}
	if p.ReauditBaseRate > 1000 {
		return fmt.Errorf("reaudit_base_rate must be <= 1000, got %d", p.ReauditBaseRate)
	}
	if p.ReauditRateMin > p.ReauditRateMax {
		return fmt.Errorf("reaudit_rate_min (%d) must not exceed reaudit_rate_max (%d)", p.ReauditRateMin, p.ReauditRateMax)
	}
	if p.ReauditTimeout <= 0 {
		return fmt.Errorf("reaudit_timeout must be positive, got %d", p.ReauditTimeout)
	}
	// S9: per-token billing param validation
	if p.DishonestJailThreshold == 0 {
		return fmt.Errorf("dishonest_jail_threshold must be positive")
	}
	return nil
}
