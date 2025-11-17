package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/github/gh-templater/internal/apply"
	"github.com/github/gh-templater/internal/github"
	"github.com/github/gh-templater/internal/runner"
)

const (
	testOrg             = "spacetj"
	testRepo            = "spacetj/gh-templater"
	testTemplate        = "templates/e2e-smoke.yaml"
	projectPrefix       = "gh-templater e2e"
	smokeLabel          = "smoke-test"
	smokeMilestoneTitle = "Smoke Cycle"
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

	cleanupSmokeArtifacts(t)
	beforeIssues := listSmokeIssues(t)

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

	milestoneNumber := lookupSmokeMilestoneNumber(t)
	if milestoneNumber == "" {
		t.Fatalf("expected %s milestone to exist", smokeMilestoneTitle)
	}
	t.Cleanup(func() {
		deleteSmokeMilestone(t, milestoneNumber)
	})

	newIssue := detectNewSmokeIssue(t, beforeIssues)
	if newIssue.ID == "" {
		t.Fatalf("expected a new smoke issue to be created")
	}
	if newIssue.Milestone == nil || newIssue.Milestone.Title != smokeMilestoneTitle {
		t.Fatalf("smoke issue missing milestone: %+v", newIssue.Milestone)
	}
	t.Cleanup(func() {
		deleteIssue(t, newIssue.ID)
	})
}

func lookupProjectID(t *testing.T, projectName string) string {
	t.Helper()
	if id := queryProjects(t, "organization", projectName); id != "" {
		return id
	}
	return queryProjects(t, "user", projectName)
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

type smokeIssue struct {
	ID        string `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
}

func listSmokeIssues(t *testing.T) map[string]smokeIssue {
	output := runGhCommand(t, "issue", "list", "--repo", testRepo, "--label", smokeLabel, "--state", "all", "--limit", "100", "--json", "id,number,title,milestone")
	if strings.TrimSpace(output) == "" {
		return map[string]smokeIssue{}
	}
	var issues []smokeIssue
	if err := json.Unmarshal([]byte(output), &issues); err != nil {
		t.Fatalf("parse smoke issues: %v", err)
	}
	result := make(map[string]smokeIssue)
	for _, issue := range issues {
		result[issue.ID] = issue
	}
	return result
}

func detectNewSmokeIssue(t *testing.T, before map[string]smokeIssue) smokeIssue {
	after := listSmokeIssues(t)
	var newIssue smokeIssue
	for id, issue := range after {
		if _, ok := before[id]; ok {
			continue
		}
		if newIssue.ID == "" || issue.Number > newIssue.Number {
			newIssue = issue
		}
	}
	return newIssue
}

func lookupSmokeMilestoneNumber(t *testing.T) string {
	output := runGhCommand(t, "api", fmt.Sprintf("repos/%s/milestones?state=all&per_page=100", testRepo))
	var milestones []struct {
		Title  string `json:"title"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal([]byte(output), &milestones); err != nil {
		t.Fatalf("parse milestones: %v", err)
	}
	for _, m := range milestones {
		if m.Title == smokeMilestoneTitle {
			return fmt.Sprintf("%d", m.Number)
		}
	}
	return ""
}

func deleteSmokeMilestone(t *testing.T, number string) {
	if number == "" {
		return
	}
	_, err := runGhCommandAllowError("api", fmt.Sprintf("repos/%s/milestones/%s", testRepo, number), "--method", "DELETE")
	if err != nil && !strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("delete milestone %s failed: %v", number, err)
	}
}

func deleteIssue(t *testing.T, issueID string) {
	if issueID == "" {
		return
	}
	const mutation = `mutation($issueId:ID!) {
  deleteIssue(input:{issueId:$issueId}) {
    clientMutationId
  }
}`
	_, err := runGhCommandAllowError("api", "graphql",
		"-f", fmt.Sprintf("query=%s", mutation),
		"-F", fmt.Sprintf("issueId=%s", issueID),
	)
	if err != nil && !strings.Contains(err.Error(), "Could not resolve to an Issue") {
		t.Fatalf("delete issue failed: %v", err)
	}
}

func cleanupSmokeArtifacts(t *testing.T) {
	issues := listSmokeIssues(t)
	for _, issue := range issues {
		deleteIssue(t, issue.ID)
	}
	if number := lookupSmokeMilestoneNumber(t); number != "" {
		deleteSmokeMilestone(t, number)
	}
}

func queryProjects(t *testing.T, ownerType, projectName string) string {
	t.Helper()
	query := fmt.Sprintf(`query($login:String!, $search:String!) {
  %s(login:$login) {
    projectsV2(first: 20, query: $search) {
      nodes { id title number }
    }
  }
}`, ownerType)

	output, err := runGhCommandAllowError("api", "graphql",
		"-f", fmt.Sprintf("query=%s", query),
		"-F", fmt.Sprintf("login=%s", testOrg),
		"-F", fmt.Sprintf("search=%s", projectName),
	)
	if err != nil {
		if strings.Contains(err.Error(), "Could not resolve to an Organization") || strings.Contains(err.Error(), "Could not resolve to a User") {
			return ""
		}
		t.Fatalf("project lookup failed: %v", err)
	}

	type node struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	var resp struct {
		Data struct {
			Organization *struct {
				Projects struct {
					Nodes []node `json:"nodes"`
				} `json:"projectsV2"`
			} `json:"organization"`
			User *struct {
				Projects struct {
					Nodes []node `json:"nodes"`
				} `json:"projectsV2"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("parse project query: %v", err)
	}

	var nodes []node
	switch ownerType {
	case "organization":
		if resp.Data.Organization != nil {
			nodes = resp.Data.Organization.Projects.Nodes
		}
	case "user":
		if resp.Data.User != nil {
			nodes = resp.Data.User.Projects.Nodes
		}
	}

	for _, n := range nodes {
		if n.Title == projectName {
			return n.ID
		}
	}
	return ""
}

func runGhCommand(t *testing.T, args ...string) string {
	t.Helper()
	output, err := runGhCommandAllowError(args...)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return output
}

func runGhCommandAllowError(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("gh %v failed: %w: %s", args, err, string(output))
	}
	return string(output), nil
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
