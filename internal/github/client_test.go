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

func TestFindProjectOrganization(t *testing.T) {
	runner := &fakeRunner{responses: []string{
		`{"data":{"organization":{"projectsV2":{"nodes":[{"id":"P1","title":"Demo","number":4,"url":"https://example.com"}]}}}}`,
	}}
	client := NewCLIClient(runner)
	project, err := client.FindProject("octo-org", "Demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.ID != "P1" || project.Number != 4 {
		t.Fatalf("unexpected project info: %+v", project)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected single query call, got %d", len(runner.calls))
	}
}

func TestFindProjectFallsBackToUser(t *testing.T) {
	runner := &fakeRunner{
		responses: []string{
			`{"data":{"user":{"projectsV2":{"nodes":[{"id":"P2","title":"Demo","number":7,"url":"https://example.com/demo"}]}}}}`,
		},
		errs: map[int]error{
			1: errors.New("Could not resolve to an Organization with the login of \"octo-user\"."),
		},
	}
	client := NewCLIClient(runner)
	project, err := client.FindProject("octo-user", "Demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.ID != "P2" || project.Number != 7 {
		t.Fatalf("unexpected project info: %+v", project)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected org + user queries")
	}
}

func TestDeleteProject(t *testing.T) {
	runner := &fakeRunner{}
	client := NewCLIClient(runner)
	if err := client.DeleteProject("PRJ_ID"); err != nil {
		t.Fatalf("unexpected error deleting project: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected mutation call")
	}
}

func TestFindMilestone(t *testing.T) {
	runner := &fakeRunner{responses: []string{`[{"title":"Smoke Cycle","number":42}]`}}
	client := NewCLIClient(runner)
	info, err := client.FindMilestone("acme/repo", "Smoke Cycle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Number != 42 {
		t.Fatalf("unexpected milestone info: %+v", info)
	}
}

func TestDeleteMilestone(t *testing.T) {
	runner := &fakeRunner{}
	client := NewCLIClient(runner)
	if err := client.DeleteMilestone("acme/repo", 5); err != nil {
		t.Fatalf("delete milestone failed: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected API call")
	}
}

func TestFindIssues(t *testing.T) {
	response := `{"data":{"search":{"nodes":[{"id":"ISSUE_ID","title":"Smoke","number":12,"url":"https://example.com/issues/12","repository":{"nameWithOwner":"acme/repo"},"milestone":{"title":"Smoke Cycle"},"labels":{"nodes":[{"name":"smoke-test"}]}}]}}}`
	runner := &fakeRunner{responses: []string{response}}
	client := NewCLIClient(runner)
	issues, err := client.FindIssues("acme/repo", "Smoke")
	if err != nil {
		t.Fatalf("find issues failed: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "ISSUE_ID" {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if issues[0].Milestone != "Smoke Cycle" || len(issues[0].Labels) != 1 {
		t.Fatalf("unexpected issue metadata: %+v", issues[0])
	}
}

func TestDeleteIssue(t *testing.T) {
	runner := &fakeRunner{}
	client := NewCLIClient(runner)
	if err := client.DeleteIssue("ISSUE_ID"); err != nil {
		t.Fatalf("delete issue failed: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected single mutation call")
	}
}

func TestDeleteLabel(t *testing.T) {
	runner := &fakeRunner{}
	client := NewCLIClient(runner)
	if err := client.DeleteLabel("acme/repo", "smoke-test"); err != nil {
		t.Fatalf("delete label failed: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected API call")
	}
}
