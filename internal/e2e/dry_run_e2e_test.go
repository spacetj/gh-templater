package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDryRunApplyAndDeleteCli(t *testing.T) {
	repoRoot := repoRoot(t)
	templatePath := filepath.Join(repoRoot, testTemplate)

	applyOutput := runTemplaterDryRun(t, repoRoot, "apply",
		"--org", "acme",
		"--project", "dry-run-preview",
		"--issues-repo", "acme/example",
		"--template", templatePath,
	)
	for _, expected := range []string{
		"[dry-run] Create project",
		"[dry-run] Ensure label",
		"[dry-run] Create milestone",
		"[dry-run] Create issue",
	} {
		if !strings.Contains(applyOutput, expected) {
			t.Fatalf("apply dry run output missing %q: %s", expected, applyOutput)
		}
	}

	deleteOutput := runTemplaterDryRun(t, repoRoot, "delete",
		"--org", "acme",
		"--project", "dry-run-preview",
		"--issues-repo", "acme/example",
		"--template", templatePath,
	)
	if !strings.Contains(deleteOutput, "[dry-run] Delete project") {
		t.Fatalf("delete dry run output missing project deletion: %s", deleteOutput)
	}
}

func runTemplaterDryRun(t *testing.T, repoRoot, subcommand string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"run", "./cmd/gh-templater", subcommand}, args...)
	cmdArgs = append(cmdArgs, "--dry-run")

	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GH_TOKEN=", "GITHUB_TOKEN=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gh-templater %s dry run failed: %v: %s", subcommand, err, string(output))
	}
	return string(output)
}
