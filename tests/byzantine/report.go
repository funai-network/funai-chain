// report.go — formatter matching KT plan §6 layout.
//
// Output goes to the test log via t.Logf so it shows up under `go test -v`
// without needing a separate output channel.

package byzantine

import (
	"fmt"
	"sort"
	"strings"
)

// Summary aggregates round results across all scenarios run in one pass.
type Summary struct {
	Results          []RoundResult
	RoundsPerScen    int
	InvariantPasses  map[string]int
	InvariantFails   map[string]int
	ScenarioPasses   map[string]int
	ScenarioFails    map[string]int
	ScenariosByTier  map[Tier][]string
	TotalAssertions  int
	TotalViolations  int
}

// Aggregate folds a flat result list into the summary structure used by the
// formatter. Caller passes `roundsPerScen` so empty scenarios still appear
// in the per-tier counts.
func Aggregate(results []RoundResult, roundsPerScen int) Summary {
	s := Summary{
		Results:         results,
		RoundsPerScen:   roundsPerScen,
		InvariantPasses: map[string]int{},
		InvariantFails:  map[string]int{},
		ScenarioPasses:  map[string]int{},
		ScenarioFails:   map[string]int{},
		ScenariosByTier: map[Tier][]string{},
	}
	for _, r := range results {
		key := r.ScenarioID
		if r.Pass() {
			s.ScenarioPasses[key]++
		} else {
			s.ScenarioFails[key]++
		}
		// Track which IDs belong to which tier (deduplicate).
		ids := s.ScenariosByTier[r.Tier]
		seen := false
		for _, id := range ids {
			if id == key {
				seen = true
				break
			}
		}
		if !seen {
			s.ScenariosByTier[r.Tier] = append(ids, key)
		}
		// Per-invariant book-keeping. `r.Invariants` is the *names* of failed
		// invariants for this round, so a successful round adds 7 passes and
		// a failure round adds (7 - len(failures)) passes + len(failures) fails.
		for _, inv := range allInvariants {
			if rContains(r.Invariants, inv.name) {
				s.InvariantFails[inv.name]++
			} else {
				s.InvariantPasses[inv.name]++
			}
		}
		s.TotalAssertions += len(allInvariants)
		s.TotalViolations += len(r.Invariants)
	}
	return s
}

func rContains(haystack []string, needle string) bool {
	for _, h := range haystack {
		// h has format "<name>: <message>"; match the prefix.
		if strings.HasPrefix(h, needle+":") {
			return true
		}
	}
	return false
}

// Format renders the summary in KT §6 layout.
func (s Summary) Format() string {
	var b strings.Builder
	b.WriteString("\n=== Byzantine Fuzzing Report ===\n")
	totalScen := 0
	for _, ids := range s.ScenariosByTier {
		totalScen += len(ids)
	}
	fmt.Fprintf(&b, "Scenarios:                 %d\n", totalScen)
	fmt.Fprintf(&b, "Rounds per scenario:       %d\n", s.RoundsPerScen)
	fmt.Fprintf(&b, "Total assertions:          %s\n\n", commaInt(s.TotalAssertions))

	for _, tier := range []Tier{TierLight, TierModerate, TierSevere, TierCombined} {
		ids := s.ScenariosByTier[tier]
		if len(ids) == 0 {
			continue
		}
		sort.Strings(ids)
		passes := 0
		total := 0
		for _, id := range ids {
			passes += s.ScenarioPasses[id]
			total += s.ScenarioPasses[id] + s.ScenarioFails[id]
		}
		// Lex sort puts C10 between C1 and C2 — show count instead of range
		// to avoid the confusing "C1-C9 (combined)" label that hides C10.
		label := fmt.Sprintf("%s tier (%d scenarios):", capitalize(string(tier)), len(ids))
		fmt.Fprintf(&b, "%-32s %s/%s PASS\n", label, commaInt(passes), commaInt(total))
	}

	fmt.Fprintf(&b, "\nInvariants checked:        %d × %d = %s\n",
		len(allInvariants), s.TotalAssertions/maxInt(len(allInvariants), 1),
		commaInt(s.TotalAssertions))
	fmt.Fprintf(&b, "Violations:                %d\n\n", s.TotalViolations)

	for _, inv := range allInvariants {
		status := "PASS"
		if s.InvariantFails[inv.name] > 0 {
			status = fmt.Sprintf("FAIL (%d)", s.InvariantFails[inv.name])
		}
		fmt.Fprintf(&b, "%-26s %s\n", invDisplayName(inv.name)+":", status)
	}

	if s.TotalViolations > 0 {
		b.WriteString("\n--- First 10 violations (with seed for repro) ---\n")
		shown := 0
		for _, r := range s.Results {
			if r.Pass() {
				continue
			}
			fmt.Fprintf(&b, "  [%s round=%d seed=%d] err=%v invariants=%v\n",
				r.ScenarioID, r.Round, r.Seed, r.Err, r.Invariants)
			shown++
			if shown >= 10 {
				break
			}
		}
	}
	return b.String()
}

func invDisplayName(name string) string {
	// fee_conservation -> Fee conservation
	parts := strings.Split(name, "_")
	parts[0] = strings.ToUpper(parts[0][:1]) + parts[0][1:]
	return strings.Join(parts, " ")
}

func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
