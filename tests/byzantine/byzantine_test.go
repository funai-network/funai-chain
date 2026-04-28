// byzantine_test.go — entrypoints for `go test`.
//
// Two run modes:
//
//   - `make test-byzantine-quick` (default, every PR) → 100 rounds per scenario
//   - `make test-byzantine-full`  (nightly, build tag) → 10 000 rounds per scenario
//
// The build tag on `byzantine_full_test.go` keeps the 10k path out of the
// default test binary so PR-time runs stay fast.

package byzantine

import (
	"testing"
)

const quickRoundsPerScenario = 100

// TestByzantineQuick runs every scenario for `quickRoundsPerScenario` rounds
// and prints the §6-style summary at the end. Any single round failure
// `t.Errorf`s with the seed for repro; the run continues so the report
// surfaces the full picture rather than halting at the first failure.
func TestByzantineQuick(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping byzantine harness in -short mode")
	}
	runAll(t, quickRoundsPerScenario)
}

func runAll(t *testing.T, rounds int) {
	t.Helper()
	scenarios := AllScenarios()
	all := make([]RoundResult, 0, len(scenarios)*rounds)
	for _, s := range scenarios {
		results := Run(t, s, rounds, false)
		all = append(all, results...)
	}
	summary := Aggregate(all, rounds)
	t.Log(summary.Format())
}
