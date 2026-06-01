package runenv

import (
	"github.com/brandonkramer/procenv"
	"github.com/brandonkramer/proctree"
)

// Lookup reads one environment variable from pid.
type Lookup func(pid int, key string) (string, bool)

// VerifyRunID reports whether pid refers to runID using env markers or fallback spec.
func VerifyRunID(keys EnvKeys, runID string, pid int, alive func(int) bool, fallback *proctree.Spec) bool {
	return verifyRunID(keys, runID, pid, alive, procenv.ProcEnv, fallback)
}

func verifyRunID(keys EnvKeys, runID string, pid int, alive func(int) bool, lookup Lookup, fallback *proctree.Spec) bool {
	if pid <= 0 {
		return false
	}
	if alive != nil && !alive(pid) {
		return false
	}
	if keys.RunID != "" {
		if v, ok := lookup(pid, keys.RunID); ok {
			return runID != "" && v == runID
		}
	}
	if fallback == nil {
		return false
	}
	return proctree.VerifyOwned(pid, fallback)
}
