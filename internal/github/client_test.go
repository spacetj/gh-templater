package github

import (
	"errors"
	"testing"
)

type fakeRunner struct {
	calls     []call
	responses []string
	errs      map[int]error
}

type call struct {
	cmd  string
	args []string
}

func (f *fakeRunner) Run(cmd string, args ...string) (string, error) {
	f.calls = append(f.calls, call{cmd: cmd, args: args})
	if f.errs != nil {
		if err, ok := f.errs[len(f.calls)]; ok {
			return "", err
		}
	}

	if len(f.responses) == 0 {
		return "", nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func TestCreateProjectOrganizationOwner(t *testing.T) {
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

func TestCreateProjectUserFallback(t *testing.T) {
	runner := &fakeRunner{
		responses: []string{
			`{"data":{"user":{"id":"USER_ID"}}}`,
			`{"data":{"createProjectV2":{"projectV2":{"id":"PROJ_ID","number":12,"url":"https://example.com"}}}}`,
		},
		errs: map[int]error{
			1: errors.New("Could not resolve to an Organization with the login of \"octo-user\"."),
		},
	}

	client := NewCLIClient(runner)
	project, err := client.CreateProject("octo-user", "Demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.ID != "PROJ_ID" {
		t.Fatalf("unexpected project info: %+v", project)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 commands (org lookup, user lookup, mutation)")
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

func TestEnsureLabelCreatesWhenMissing(t *testing.T) {
	runner := &fakeRunner{
		errs: map[int]error{
			1: errors.New("HTTP 404: Not Found"),
		},
	}
	client := NewCLIClient(runner)
	if err := client.EnsureLabel("acme/repo", "spec", "#ABCDEF", "desc"); err != nil {
		t.Fatalf("ensure label failed: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected PATCH + POST, got %d", len(runner.calls))
	}
	found := false
	for _, arg := range runner.calls[1].args {
		if arg == "color=abcdef" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected sanitized color in create call: %+v", runner.calls[1].args)
	}
}
