package apply

import (
	"bytes"
	"errors"
	"testing"
)

func TestStepRunnerDryRunSkipsActionAndPrints(t *testing.T) {
	var buf bytes.Buffer
	runner := newStepRunner(true, &buf)

	called := false
	if err := runner.Run("example action", func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if called {
		t.Fatalf("expected action to be skipped in dry run")
	}
	if got := buf.String(); got == "" || got != "[dry-run] example action\n" {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestStepRunnerExecutesActionWhenEnabled(t *testing.T) {
	runner := newStepRunner(false, nil)

	called := false
	errSentinel := errors.New("boom")
	if err := runner.Run("", func() error {
		called = true
		return errSentinel
	}); !errors.Is(err, errSentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if !called {
		t.Fatalf("expected action to run when not in dry run")
	}
}
