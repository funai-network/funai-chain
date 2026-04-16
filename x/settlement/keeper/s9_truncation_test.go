package keeper_test

import (
	"math"
	"testing"

	"github.com/funai-wiki/funai-chain/x/settlement/keeper"
)

// TestTruncation_IgnoredCap (TR3): Worker ignores budget truncation and generates
// more tokens than the budget allows. Settlement layer must cap at max_fee.
// This verifies CalculatePerTokenFee's max_fee cap (S9 §4.1 line: actualFee > maxFee → maxFee).
func TestTruncation_IgnoredCap(t *testing.T) {
	// Worker was supposed to stop at ~95 output tokens but generated 500
	inputTokens := uint32(100)
	outputTokens := uint32(500) // Way over budget
	feePerInput := uint64(100)  // 100 ufai/token
	feePerOutput := uint64(200) // 200 ufai/token
	maxFee := uint64(50000)     // 50000 ufai cap

	// actual = 100*100 + 500*200 = 10000 + 100000 = 110000 > 50000
	result := keeper.CalculatePerTokenFee(inputTokens, outputTokens, feePerInput, feePerOutput, maxFee)
	if result != maxFee {
		t.Fatalf("expected fee capped at max_fee=%d, got %d", maxFee, result)
	}
}

// TestTruncation_UnderBudget: actual cost < max_fee → charge actual.
func TestTruncation_UnderBudget(t *testing.T) {
	inputTokens := uint32(50)
	outputTokens := uint32(20)
	feePerInput := uint64(100)
	feePerOutput := uint64(200)
	maxFee := uint64(50000)

	// actual = 50*100 + 20*200 = 5000 + 4000 = 9000 < 50000
	result := keeper.CalculatePerTokenFee(inputTokens, outputTokens, feePerInput, feePerOutput, maxFee)
	expected := uint64(9000)
	if result != expected {
		t.Fatalf("expected actual_fee=%d, got %d", expected, result)
	}
}

// TestTruncation_ExactMaxFee: actual cost exactly equals max_fee.
func TestTruncation_ExactMaxFee(t *testing.T) {
	inputTokens := uint32(100)
	outputTokens := uint32(200)
	feePerInput := uint64(100)
	feePerOutput := uint64(200)
	maxFee := uint64(50000) // 100*100 + 200*200 = 10000+40000 = 50000

	result := keeper.CalculatePerTokenFee(inputTokens, outputTokens, feePerInput, feePerOutput, maxFee)
	if result != maxFee {
		t.Fatalf("expected exact max_fee=%d, got %d", maxFee, result)
	}
}

// TestTruncation_OverflowProtection (S9 §4.5): uint64 overflow → cap at max_fee.
func TestTruncation_OverflowProtection(t *testing.T) {
	inputTokens := uint32(1)
	outputTokens := uint32(3)
	feePerInput := uint64(1)
	feePerOutput := uint64(math.MaxUint64 / 2) // will overflow on 3 * (MaxUint64/2)
	maxFee := uint64(999999)

	result := keeper.CalculatePerTokenFee(inputTokens, outputTokens, feePerInput, feePerOutput, maxFee)
	if result != maxFee {
		t.Fatalf("expected overflow protection → max_fee=%d, got %d", maxFee, result)
	}
}

// TestTruncation_AdditionOverflow: input+output overflows uint64 → cap at max_fee.
func TestTruncation_AdditionOverflow(t *testing.T) {
	inputTokens := uint32(1)
	outputTokens := uint32(1)
	feePerInput := uint64(math.MaxUint64 - 1) // just under max
	feePerOutput := uint64(math.MaxUint64 - 1)
	maxFee := uint64(100)

	// inputCost = MaxUint64-1, outputCost = MaxUint64-1, sum overflows
	result := keeper.CalculatePerTokenFee(inputTokens, outputTokens, feePerInput, feePerOutput, maxFee)
	if result != maxFee {
		t.Fatalf("expected addition overflow → max_fee=%d, got %d", maxFee, result)
	}
}

// TestTruncation_ZeroTokens: 0 output tokens → only input cost charged.
func TestTruncation_ZeroTokens(t *testing.T) {
	result := keeper.CalculatePerTokenFee(100, 0, 10, 200, 50000)
	expected := uint64(100 * 10) // only input cost
	if result != expected {
		t.Fatalf("expected input-only fee=%d, got %d", expected, result)
	}
}
