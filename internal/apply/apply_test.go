package apply

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/github/gh-templater/internal/github"
)

type fakeClient struct {
	createdProject    github.ProjectInfo
	createdIssues     []string
	milestones        []string
	addedToProject    []string
	labels            []string
	projectFields     map[string]github.ProjectField
	fieldUpdates      []map[string]interface{}
	findProject       github.ProjectInfo
	findProjectErr    error
	deletedProjects   []string
	issueCatalog      map[string][]github.IssueInfo
	milestoneCatalog  map[string]github.MilestoneInfo
	deletedIssues     []string
	deletedMilestones []int
	deletedLabels     []string
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

func (f *fakeClient) FindProject(owner, title string) (github.ProjectInfo, error) {
	if f.findProjectErr != nil {
		return github.ProjectInfo{}, f.findProjectErr
	}
	return f.findProject, nil
}

func (f *fakeClient) DeleteProject(projectID string) error {
	f.deletedProjects = append(f.deletedProjects, projectID)
	return nil
}

func (f *fakeClient) FindMilestone(repo, title string) (github.MilestoneInfo, error) {
	if info, ok := f.milestoneCatalog[title]; ok {
		return info, nil
	}
	return github.MilestoneInfo{}, fmt.Errorf("%w", github.ErrMilestoneNotFound)
}

func (f *fakeClient) DeleteMilestone(repo string, number int) error {
	f.deletedMilestones = append(f.deletedMilestones, number)
	return nil
}

func (f *fakeClient) FindIssues(repo, title string) ([]github.IssueInfo, error) {
	if f.issueCatalog == nil {
		return nil, nil
	}
	return f.issueCatalog[title], nil
}

func (f *fakeClient) DeleteIssue(issueID string) error {
	f.deletedIssues = append(f.deletedIssues, issueID)
	return nil
}

func (f *fakeClient) DeleteLabel(repo, name string) error {
	f.deletedLabels = append(f.deletedLabels, repo+":"+name)
	return nil
}

func TestApplyValidTemplate(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `name: Demo
project:
  readme: some readme
  fields:
    - name: Status
      data_type: TEXT
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

func TestApplySkipsProjectWhenTemplateMissingProjectBlock(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `name: Demo
labels:
  - name: docs
    color: "#123456"
`
	writeFile(tplPath, content, t)
	client := &fakeClient{}
	opts := Options{Org: "acme", ProjectName: "Demo", IssuesRepo: "acme/repo", Template: tplPath}
	if err := Apply(opts, client); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if client.createdProject.ID != "" {
		t.Fatalf("expected project creation to be skipped when template lacks project block")
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
project:
  readme: README
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

func TestBuildFieldValue(t *testing.T) {
	singleSelectField := github.ProjectField{
		DataType: "SINGLE_SELECT",
		Options: map[string]github.ProjectFieldOption{
			"High": {ID: "opt-high", Name: "High"},
		},
	}
	tests := []struct {
		name    string
		field   github.ProjectField
		input   string
		want    map[string]interface{}
		wantErr bool
	}{
		{name: "text", field: github.ProjectField{DataType: "TEXT"}, input: "ready", want: map[string]interface{}{"text": "ready"}},
		{name: "number", field: github.ProjectField{DataType: "NUMBER"}, input: "42.5", want: map[string]interface{}{"number": 42.5}},
		{name: "blank number", field: github.ProjectField{DataType: "NUMBER"}, input: " ", want: map[string]interface{}{"number": 0.0}},
		{name: "invalid number", field: github.ProjectField{DataType: "NUMBER"}, input: "abc", wantErr: true},
		{name: "date", field: github.ProjectField{DataType: "DATE"}, input: "2024-01-02", want: map[string]interface{}{"date": "2024-01-02"}},
		{name: "single select", field: singleSelectField, input: "High", want: map[string]interface{}{"singleSelectOptionId": "opt-high"}},
		{name: "missing option", field: singleSelectField, input: "Low", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildFieldValue(tt.field, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected field value: %+v", got)
			}
		})
	}
}

func TestApplyIssueFieldsValidation(t *testing.T) {
	client := &fakeClient{}
	fields := map[string]github.ProjectField{
		"Spec": {ID: "f1", Name: "Spec", DataType: "TEXT", Options: map[string]github.ProjectFieldOption{}},
	}
	if err := applyIssueFields(map[string]string{"unknown": "foo"}, "proj", "item", fields, client); err == nil {
		t.Fatalf("expected error for missing field metadata")
	}
	if err := applyIssueFields(map[string]string{"Spec": "ready"}, "proj", "item", nil, client); err == nil {
		t.Fatalf("expected error when no project fields available")
	}
	if err := applyIssueFields(map[string]string{"Spec": "ready"}, "proj", "item", fields, client); err != nil {
		t.Fatalf("unexpected error applying field: %v", err)
	}
	if len(client.fieldUpdates) != 1 {
		t.Fatalf("expected a single field update, got %d", len(client.fieldUpdates))
	}
	if !reflect.DeepEqual(client.fieldUpdates[0], map[string]interface{}{"text": "ready"}) {
		t.Fatalf("unexpected update payload: %+v", client.fieldUpdates[0])
	}
}

func TestDeleteProject(t *testing.T) {
	client := &fakeClient{findProject: github.ProjectInfo{ID: "proj-123", URL: "https://example.com/project/proj-123"}}
	opts := DeleteOptions{Org: "acme", ProjectName: "Demo", Sections: Sections{Project: true}}
	if err := Delete(opts, client); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if len(client.deletedProjects) != 1 || client.deletedProjects[0] != "proj-123" {
		t.Fatalf("expected project deletion, got %+v", client.deletedProjects)
	}
}

func TestDeleteProjectPropagatesLookupError(t *testing.T) {
	client := &fakeClient{findProjectErr: fmt.Errorf("project not found")}
	opts := DeleteOptions{Org: "acme", ProjectName: "Missing", Sections: Sections{Project: true}}
	if err := Delete(opts, client); err == nil {
		t.Fatalf("expected error when project lookup fails")
	}
}

func TestDeleteIssuesAndMilestones(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `name: Demo
milestones:
  - title: Smoke Cycle
issues:
  - title: Smoke Issue
    labels: [smoke-test]
    milestone: Smoke Cycle
`
	writeFile(tplPath, content, t)
	client := &fakeClient{
		issueCatalog: map[string][]github.IssueInfo{
			"Smoke Issue": {
				{ID: "issue-1", Title: "Smoke Issue", Labels: []string{"smoke-test"}, Milestone: "Smoke Cycle"},
			},
		},
		milestoneCatalog: map[string]github.MilestoneInfo{
			"Smoke Cycle": {Number: 42, Title: "Smoke Cycle"},
		},
	}
	opts := DeleteOptions{IssuesRepo: "acme/repo", Template: tplPath, Sections: Sections{Issues: true, Milestones: true}}
	if err := Delete(opts, client); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if len(client.deletedIssues) != 1 || client.deletedIssues[0] != "issue-1" {
		t.Fatalf("expected issue deletion, got %+v", client.deletedIssues)
	}
	if len(client.deletedMilestones) != 1 || client.deletedMilestones[0] != 42 {
		t.Fatalf("expected milestone deletion, got %+v", client.deletedMilestones)
	}
}

func TestDeleteLabels(t *testing.T) {
	tplPath := t.TempDir() + "/template.yaml"
	content := `labels:
  - name: smoke-test
`
	writeFile(tplPath, content, t)
	client := &fakeClient{}
	opts := DeleteOptions{IssuesRepo: "acme/repo", Template: tplPath, Sections: Sections{Labels: true}}
	if err := Delete(opts, client); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if len(client.deletedLabels) != 1 || client.deletedLabels[0] != "acme/repo:smoke-test" {
		t.Fatalf("expected label deletion, got %+v", client.deletedLabels)
	}
}

func writeFile(path, content string, t *testing.T) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
}
