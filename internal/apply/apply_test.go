package apply

import (
	"os"
	"testing"

	"github.com/github/gh-templater/internal/github"
)

type fakeClient struct {
	createdProject github.ProjectInfo
	createdIssues  []string
	milestones     []string
	addedToProject []string
}

func (f *fakeClient) CreateProject(owner, title string) (github.ProjectInfo, error) {
	f.createdProject = github.ProjectInfo{ID: "p123", Number: 2, URL: "https://example.com"}
	return f.createdProject, nil
}

func (f *fakeClient) UpdateProjectReadme(projectID, readme string) error { return nil }

func (f *fakeClient) CreateMilestone(repo string, milestoneTitle, description, dueOn string) error {
	f.milestones = append(f.milestones, milestoneTitle)
	return nil
}

func (f *fakeClient) CreateIssue(repo string, issue github.TemplateIssueWithResolvedMilestone) (string, error) {
	url := "https://example.com/issues/" + issue.Title
	f.createdIssues = append(f.createdIssues, url)
	return url, nil
}

func (f *fakeClient) AddItemToProject(owner string, projectNumber int, itemURL string) error {
	f.addedToProject = append(f.addedToProject, itemURL)
	return nil
}

func TestApplyValidTemplate(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `name: Demo
readme: some readme
milestones:
  - title: Kickoff
issues:
  - title: First
    body: Details
    milestone: Kickoff
`
	writeFile(tplPath, content, t)

	client := &fakeClient{}
	opts := Options{Org: "acme", ProjectName: "Demo Project", IssuesRepo: "acme/repo", Template: tplPath}
	if err := Apply(opts, client); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if len(client.milestones) != 1 || client.milestones[0] != "Kickoff" {
		t.Fatalf("milestones not created correctly: %+v", client.milestones)
	}

	if len(client.createdIssues) != 1 {
		t.Fatalf("expected 1 issue created, got %d", len(client.createdIssues))
	}
}

func TestApplyMissingMilestone(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `name: Demo
issues:
  - title: First
    body: Details
    milestone: Missing
`
	writeFile(tplPath, content, t)

	client := &fakeClient{}
	opts := Options{Org: "acme", ProjectName: "Demo Project", IssuesRepo: "acme/repo", Template: tplPath}
	if err := Apply(opts, client); err == nil {
		t.Fatalf("expected error for missing milestone")
	}
}

func writeFile(path, content string, t *testing.T) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
}
