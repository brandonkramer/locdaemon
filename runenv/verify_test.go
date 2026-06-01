package runenv

import (
	"testing"

	"github.com/brandonkramer/proctree"
)

func TestVerifyRunIDEnvAndFallback(t *testing.T) {
	t.Parallel()

	keys := EnvKeys{RunID: "RUN_ID"}
	lookup := func(_ int, _ string) (string, bool) { return "run-1", true }
	if !verifyRunID(keys, "run-1", 1, func(int) bool { return true }, lookup, nil) {
		t.Fatal("expected env match")
	}
	if verifyRunID(keys, "", 1, func(int) bool { return true }, lookup, nil) {
		t.Fatal("empty run id should fail")
	}
	if verifyRunID(keys, "other", 1, func(int) bool { return true }, lookup, nil) {
		t.Fatal("mismatch should fail")
	}
	lookup = func(_ int, _ string) (string, bool) { return "", false }
	if verifyRunID(keys, "x", 1, func(int) bool { return true }, lookup, nil) {
		t.Fatal("nil fallback")
	}
	_ = verifyRunID(keys, "x", 1, func(int) bool { return true }, lookup, &proctree.Spec{Shell: "true"})
}

func TestVerifyRunIDBasics(t *testing.T) {
	t.Parallel()

	keys := EnvKeys{RunID: "RUN_ID"}
	if VerifyRunID(keys, "x", 0, nil, nil) {
		t.Fatal("pid 0 should fail")
	}
	if VerifyRunID(keys, "x", 1, func(int) bool { return false }, nil) {
		t.Fatal("not alive should fail")
	}
}
