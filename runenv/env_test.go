package runenv_test

import (
	"testing"

	"github.com/brandonkramer/locdaemon/runenv"
)

func TestChildEnv(t *testing.T) {
	t.Parallel()
	keys := runenv.EnvKeys{RunID: "APP_RUN_ID", Home: "APP_HOME"}
	env := runenv.ChildEnv(keys, "/home", "run-1", map[string]string{"K": "V"})
	found := map[string]bool{}
	for _, e := range env {
		switch e {
		case "APP_RUN_ID=run-1":
			found["run"] = true
		case "APP_HOME=/home":
			found["home"] = true
		case "K=V":
			found["extra"] = true
		}
	}
	for _, k := range []string{"run", "home", "extra"} {
		if !found[k] {
			t.Fatalf("missing %s in env=%v", k, env)
		}
	}
}

func TestChildEnvOmitsEmptyKeys(t *testing.T) {
	t.Parallel()
	env := runenv.ChildEnv(runenv.EnvKeys{}, "/home", "run-1", nil)
	for _, e := range env {
		if e == "=/home" || e == "=run-1" {
			t.Fatalf("unexpected entry %q", e)
		}
	}
}
