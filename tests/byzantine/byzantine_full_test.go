//go:build byzantine_full

// byzantine_full_test.go — nightly 10 000-rounds-per-scenario run.
//
// Excluded from the default test binary by build tag so PR-time CI stays
// fast. Run with `go test -tags byzantine_full ./tests/byzantine/...` or
// the `make test-byzantine-full` target.

package byzantine

import "testing"

const fullRoundsPerScenario = 10_000

func TestByzantineFull(t *testing.T) {
	runAll(t, fullRoundsPerScenario)
}
