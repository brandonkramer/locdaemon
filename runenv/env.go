// Package runenv builds child process environments for local daemons.
package runenv

import "os"

// EnvKeys names environment variables injected into spawned child processes.
type EnvKeys struct {
	// RunID is the variable name for a run identifier (e.g. "MYAPP_RUN_ID").
	RunID string
	// Home is the variable name for a daemon home directory (e.g. "MYAPP_HOME").
	Home string
}

// ChildEnv returns os.Environ plus configured RunID and Home keys and extra entries.
func ChildEnv(keys EnvKeys, home, runID string, extra map[string]string) []string {
	base := os.Environ()
	add := len(extra)
	if keys.RunID != "" {
		add++
	}
	if keys.Home != "" {
		add++
	}
	out := make([]string, 0, len(base)+add)
	out = append(out, base...)
	if keys.RunID != "" {
		out = append(out, keys.RunID+"="+runID)
	}
	if keys.Home != "" {
		out = append(out, keys.Home+"="+home)
	}
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}
