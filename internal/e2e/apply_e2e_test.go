package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/github/gh-templater/internal/apply"
	"github.com/github/gh-templater/internal/github"
	"github.com/github/gh-templater/internal/runner"
)

const (
	testOrg       = "spacetj"
	testRepo      = "spacetj/gh-templater"
	testTemplate  = "templates/e2e-smoke.yaml"
	projectPrefix = "gh-templater e2e"
)

// TestApplyTemplateE2E exercises gh-templater against the real GitHub APIs.
// It requires GH_TEMPLATER_TOKEN to be set with sufficient scopes and the gh CLI available.
func TestApplyTemplateE2E(t *testing.T) {
	token := os.Getenv("GH_TEMPLATER_TOKEN")
	if token == "" {
		t.Skip("GH_TEMPLATER_TOKEN not set; skipping end-to-end test")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skipf("gh CLI not available: %v", err)
	}

	if err := os.Setenv("GH_TOKEN", token); err != nil {
		t.Fatalf("set GH_TOKEN: %v", err)
	}
	if err := os.Setenv("GITHUB_TOKEN", token); err != nil {
		t.Fatalf("set GITHUB_TOKEN: %v", err)
	}

	projectName := fmt.Sprintf("%s %d", projectPrefix, time.Now().UnixNano())
	client := github.NewCLIClient(runner.ExecRunner{})
	opts := apply.Options{
		Org:         testOrg,
		ProjectName: projectName,
		IssuesRepo:  testRepo,
		Template:    filepath.Join(repoRoot(t), testTemplate),
	}

	if err := apply.Apply(opts, client); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	projectID := lookupProjectID(t, projectName)
	if projectID == "" {
		t.Fatalf("project %q not found after creation", projectName)
	}
	t.Cleanup(func() {
		deleteProject(t, projectID)
	})
}

func lookupProjectID(t *testing.T, projectName string) string {
	t.Helper()
	const query = `query($login:String!, $search:String!) {
	  organization(login:$login) {
	    projectsV2(first: 20, query: $search) {
	      nodes { id title number }
	    }
	  }
	}`
	output := runGhCommand(t, "api", "graphql",
		"-f", fmt.Sprintf("query=%s", query),
		"-F", fmt.Sprintf("login=%s", testOrg),
		"-F", fmt.Sprintf("search=%s", projectName),
	)

	var resp struct {
		Data struct {
			Organization struct {
				Projects struct {
					Nodes []struct {
						ID    string `json:"id"`
						Title string `json:"title"`
					} `json:"nodes"`
				} `json:"projectsV2"`
			} `json:"organization"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("parse project query: %v", err)
	}
	for _, node := range resp.Data.Organization.Projects.Nodes {
		if node.Title == projectName {
			return node.ID
		}
	}
	return ""
}

func deleteProject(t *testing.T, projectID string) {
	t.Helper()
	const mutation = `mutation($projectId:ID!) {
	  deleteProjectV2(input:{projectId:$projectId}) {
	    clientMutationId
	  }
	}`
	_ = runGhCommand(t, "api", "graphql",
		"-f", fmt.Sprintf("query=%s", mutation),
		"-F", fmt.Sprintf("projectId=%s", projectID),
	)
}

func runGhCommand(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("gh", args...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gh %v failed: %v\n%s", args, err, string(output))
	}
	return string(output)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
