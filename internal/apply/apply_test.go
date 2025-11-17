package apply

import (
	"fmt"
	"os"
	"testing"

	"github.com/github/gh-templater/internal/github"
)

type fakeClient struct {
	createdProject github.ProjectInfo
	createdIssues  []string
	milestones     []string
	addedToProject []string
	labels         []string
	projectFields  map[string]github.ProjectField
	fieldUpdates   []map[string]interface{}
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

func (f *fakeClient) AddItemToProject(owner string, projectNumber int, itemURL string) (string, error) {
	f.addedToProject = append(f.addedToProject, itemURL)
	return fmt.Sprintf("item-%d", len(f.addedToProject)), nil
}

func (f *fakeClient) EnsureLabel(repo, name, color, description string) error {
	f.labels = append(f.labels, name+":"+color)
	return nil
}

func (f *fakeClient) EnsureProjectFields(projectID string, fields []github.FieldTemplate) (map[string]github.ProjectField, error) {
	if f.projectFields == nil {
		f.projectFields = make(map[string]github.ProjectField)
	}
	for _, field := range fields {
		if _, ok := f.projectFields[field.Name]; ok {
			continue
		}
		options := make(map[string]github.ProjectFieldOption)
		for _, opt := range field.Options {
			options[opt.Name] = github.ProjectFieldOption{ID: opt.Name + "-id", Name: opt.Name}
		}
		f.projectFields[field.Name] = github.ProjectField{ID: field.Name + "-id", Name: field.Name, DataType: field.DataType, Options: options}
	}
	return f.projectFields, nil
}

func (f *fakeClient) UpdateProjectItemField(projectID, itemID string, fieldID string, value map[string]interface{}) error {
	f.fieldUpdates = append(f.fieldUpdates, value)
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

func TestApplyLabelsOnly(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `labels:
  - name: spec
    color: "#123abc"
`
	writeFile(tplPath, content, t)
	client := &fakeClient{}
	opts := Options{Org: "acme", ProjectName: "Demo", IssuesRepo: "acme/repo", Template: tplPath, Sections: Sections{Labels: true}}
	if err := Apply(opts, client); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if len(client.labels) != 1 {
		t.Fatalf("expected 1 label ensured, got %d", len(client.labels))
	}
	if client.createdProject.ID != "" {
		t.Fatalf("project should not be created when project section disabled")
	}
}

func TestApplyIssuesWithoutProject(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `milestones:
  - title: Kickoff
issues:
  - title: First
    body: Details
    milestone: Kickoff
`
	writeFile(tplPath, content, t)
	client := &fakeClient{}
	opts := Options{Org: "acme", ProjectName: "Demo", IssuesRepo: "acme/repo", Template: tplPath, Sections: Sections{Issues: true}}
	if err := Apply(opts, client); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if len(client.addedToProject) != 0 {
		t.Fatalf("issues should not be added to project when project section disabled")
	}
	if len(client.createdIssues) != 1 {
		t.Fatalf("expected issue creation even without project")
	}
}

func TestParseSections(t *testing.T) {
	sections, err := ParseSections("labels, issues")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !sections.Labels || !sections.Issues || sections.Project {
		t.Fatalf("unexpected sections parsed: %+v", sections)
	}
	if _, err := ParseSections("foo"); err == nil {
		t.Fatalf("expected error for unknown section")
	}
}

func TestApplySetsIssueFields(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `name: Demo
fields:
  - name: Priority
    data_type: SINGLE_SELECT
    options:
      - name: High
milestones:
  - title: Cycle
issues:
  - title: Work
    body: Details
    milestone: Cycle
    fields:
      Priority: High
`
	writeFile(tplPath, content, t)
	client := &fakeClient{}
	opts := Options{Org: "acme", ProjectName: "Demo", IssuesRepo: "acme/repo", Template: tplPath}
	if err := Apply(opts, client); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if len(client.fieldUpdates) == 0 {
		t.Fatalf("expected field updates to occur")
	}
	if _, ok := client.projectFields["Priority"]; !ok {
		t.Fatalf("expected Priority field to be created")
	}
}

func writeFile(path, content string, t *testing.T) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
}
