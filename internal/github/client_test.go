package github

import (
	"fmt"
	"testing"
)

type fakeRunner struct {
	calls     []call
	responses []string
	errAt     int
}

type call struct {
	cmd  string
	args []string
}

func (f *fakeRunner) Run(cmd string, args ...string) (string, error) {
	f.calls = append(f.calls, call{cmd: cmd, args: args})
	if f.errAt > 0 && len(f.calls) == f.errAt {
		return "", fmt.Errorf("forced error")
	}

	if len(f.responses) == 0 {
		return "", nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func TestCreateProject(t *testing.T) {
	runner := &fakeRunner{responses: []string{
		`{"data":{"organization":{"id":"ORG_ID"}}}`,
		`{"data":{"createProjectV2":{"projectV2":{"id":"PROJ_ID","number":12,"url":"https://example.com"}}}}`,
	}}

	client := NewCLIClient(runner)
	project, err := client.CreateProject("octo-org", "Demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if project.ID != "PROJ_ID" || project.Number != 12 {
		t.Fatalf("unexpected project info: %+v", project)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(runner.calls))
	}
}

func TestUpdateProjectReadmeSkipsEmpty(t *testing.T) {
	runner := &fakeRunner{}
	client := NewCLIClient(runner)

	if err := client.UpdateProjectReadme("id", "   "); err != nil {
		t.Fatalf("expected no error for empty readme, got %v", err)
	}

	if len(runner.calls) != 0 {
		t.Fatalf("expected no commands when readme is empty")
	}
}

func TestCreateIssue(t *testing.T) {
	runner := &fakeRunner{responses: []string{"https://example.com/issue/1"}}
	client := NewCLIClient(runner)

	issueURL, err := client.CreateIssue("octo/repo", TemplateIssueWithResolvedMilestone{Title: "Issue", Body: "Details", Milestone: "Sprint 1", Labels: []string{"backend"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if issueURL != "https://example.com/issue/1" {
		t.Fatalf("unexpected issue url: %s", issueURL)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}
}
