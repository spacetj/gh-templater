package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/github/gh-templater/internal/runner"
)

// Client describes the operations required to create projects, milestones, and issues.
type Client interface {
	CreateProject(owner, title string) (ProjectInfo, error)
	UpdateProjectReadme(projectID, readme string) error
	CreateMilestone(repo string, milestoneTitle, description, dueOn string) error
	CreateIssue(repo string, issue TemplateIssueWithResolvedMilestone) (string, error)
	AddItemToProject(owner string, projectNumber int, itemURL string) (string, error)
	EnsureLabel(repo, name, color, description string) error
	EnsureProjectFields(projectID string, fields []FieldTemplate) (map[string]ProjectField, error)
	UpdateProjectItemField(projectID, itemID, fieldID string, value map[string]interface{}) error
}

type FieldTemplate struct {
	Name        string
	DataType    string
	Description string
	Options     []FieldOption
}

type FieldOption struct {
	Name        string
	Color       string
	Description string
}

type ProjectField struct {
	ID       string
	Name     string
	DataType string
	Options  map[string]ProjectFieldOption
}

type ProjectFieldOption struct {
	ID   string
	Name string
}

// TemplateIssueWithResolvedMilestone includes milestone information after validation.
type TemplateIssueWithResolvedMilestone struct {
	Title     string
	Body      string
	Labels    []string
	Milestone string
	Assignees []string
}

// ProjectInfo captures identifiers returned from GitHub.
type ProjectInfo struct {
	ID     string
	Number int
	URL    string
}

// CLIClient implements Client using the GitHub CLI.
type CLIClient struct {
	runner runner.Runner
}

// NewCLIClient constructs a new GitHub CLI-backed client.
func NewCLIClient(r runner.Runner) *CLIClient {
	return &CLIClient{runner: r}
}

// CreateProject creates a project for the organization and returns its identifiers.
func (c *CLIClient) CreateProject(owner, title string) (ProjectInfo, error) {
	ownerID, err := c.lookupOwnerID(owner)
	if err != nil {
		return ProjectInfo{}, err
	}

	query := `mutation($ownerId:ID!, $title:String!) { createProjectV2(input:{ownerId:$ownerId, title:$title}) { projectV2 { id number url } } }`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-F", "ownerId="+ownerID, "-F", "title="+title)
	if err != nil {
		return ProjectInfo{}, fmt.Errorf("create project: %w", err)
	}

	var parsed struct {
		Data struct {
			CreateProjectV2 struct {
				ProjectV2 ProjectInfo `json:"projectV2"`
			} `json:"createProjectV2"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return ProjectInfo{}, fmt.Errorf("parse create project response: %w", err)
	}

	return parsed.Data.CreateProjectV2.ProjectV2, nil
}

// UpdateProjectReadme updates the project's README content.
func (c *CLIClient) UpdateProjectReadme(projectID, readme string) error {
	if strings.TrimSpace(readme) == "" {
		return nil
	}

	mutation := `mutation($projectId:ID!, $readme:String!) { updateProjectV2(input:{projectId:$projectId, readme:$readme}) { projectV2 { id } } }`
	if _, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+mutation, "-F", "projectId="+projectID, "--raw-field", "readme="+readme); err != nil {
		return fmt.Errorf("update project readme: %w", err)
	}
	return nil
}

// CreateMilestone creates a milestone in the provided repository (owner/repo).
func (c *CLIClient) CreateMilestone(repo string, milestoneTitle, description, dueOn string) error {
	args := []string{"api", "repos/" + repo + "/milestones", "--method", "POST", "-f", "title=" + milestoneTitle}
	if strings.TrimSpace(description) != "" {
		args = append(args, "--raw-field", "description="+description)
	}
	if strings.TrimSpace(dueOn) != "" {
		args = append(args, "-f", "due_on="+dueOn)
	}

	if _, err := c.runner.Run("gh", args...); err != nil {
		return fmt.Errorf("create milestone %q: %w", milestoneTitle, err)
	}
	return nil
}

// CreateIssue creates an issue and returns the created issue URL.
func (c *CLIClient) CreateIssue(repo string, issue TemplateIssueWithResolvedMilestone) (string, error) {
	args := []string{"issue", "create", "--repo", repo, "--title", issue.Title, "--body", issue.Body}
	for _, label := range issue.Labels {
		args = append(args, "--label", label)
	}
	if issue.Milestone != "" {
		args = append(args, "--milestone", issue.Milestone)
	}
	for _, assignee := range issue.Assignees {
		args = append(args, "--assignee", assignee)
	}

	output, err := c.runner.Run("gh", args...)
	if err != nil {
		return "", fmt.Errorf("create issue %q: %w", issue.Title, err)
	}

	return strings.TrimSpace(output), nil
}

// AddItemToProject adds an issue or pull request to the project.
func (c *CLIClient) AddItemToProject(owner string, projectNumber int, itemURL string) (string, error) {
	args := []string{"project", "item-add", fmt.Sprintf("%d", projectNumber), "--owner", owner, "--url", itemURL, "--format", "json"}
	output, err := c.runner.Run("gh", args...)
	if err != nil {
		return "", fmt.Errorf("add item to project: %w", err)
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return "", fmt.Errorf("parse project item: %w", err)
	}
	return resp.ID, nil
}

var (
	errOrganizationNotFound = errors.New("organization not found")
	errUserNotFound         = errors.New("user not found")
)

func (c *CLIClient) lookupOrganizationID(owner string) (string, error) {
	query := `query($login:String!){ organization(login:$login) { id } }`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-F", "login="+owner)
	if err != nil {
		if strings.Contains(err.Error(), "Could not resolve to an Organization") {
			return "", fmt.Errorf("%w: %s", errOrganizationNotFound, owner)
		}
		return "", fmt.Errorf("lookup organization: %w", err)
	}

	var parsed struct {
		Data struct {
			Organization struct {
				ID string `json:"id"`
			} `json:"organization"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return "", fmt.Errorf("parse organization id: %w", err)
	}

	if parsed.Data.Organization.ID == "" {
		return "", fmt.Errorf("%w: %s", errOrganizationNotFound, owner)
	}

	return parsed.Data.Organization.ID, nil
}

func (c *CLIClient) lookupOwnerID(owner string) (string, error) {
	id, err := c.lookupOrganizationID(owner)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, errOrganizationNotFound) {
		return "", err
	}
	return c.lookupUserID(owner)
}

func (c *CLIClient) lookupUserID(owner string) (string, error) {
	query := `query($login:String!){ user(login:$login) { id } }`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-F", "login="+owner)
	if err != nil {
		if strings.Contains(err.Error(), "Could not resolve to a User") {
			return "", fmt.Errorf("%w: %s", errUserNotFound, owner)
		}
		return "", fmt.Errorf("lookup user: %w", err)
	}

	var parsed struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return "", fmt.Errorf("parse user id: %w", err)
	}
	if parsed.Data.User.ID == "" {
		return "", fmt.Errorf("%w: %s", errUserNotFound, owner)
	}
	return parsed.Data.User.ID, nil
}

func (c *CLIClient) EnsureLabel(repo, name, color, description string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("label name is required")
	}
	color = sanitizeColor(color)
	path := fmt.Sprintf("repos/%s/labels/%s", repo, url.PathEscape(name))
	args := []string{"api", path, "--method", "PATCH", "-f", "name=" + name, "-f", "color=" + color}
	if strings.TrimSpace(description) != "" {
		args = append(args, "--raw-field", "description="+description)
	}
	if _, err := c.runner.Run("gh", args...); err != nil {
		if !isNotFound(err) {
			return fmt.Errorf("update label %q: %w", name, err)
		}
		createArgs := []string{"api", fmt.Sprintf("repos/%s/labels", repo), "--method", "POST", "-f", "name=" + name, "-f", "color=" + color}
		if strings.TrimSpace(description) != "" {
			createArgs = append(createArgs, "--raw-field", "description="+description)
		}
		if _, err := c.runner.Run("gh", createArgs...); err != nil {
			return fmt.Errorf("create label %q: %w", name, err)
		}
	}
	return nil
}

func sanitizeColor(color string) string {
	trimmed := strings.TrimSpace(color)
	trimmed = strings.TrimPrefix(trimmed, "#")
	if trimmed == "" {
		return "ededed"
	}
	return strings.ToLower(trimmed)
}

func isNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Not Found") || strings.Contains(msg, "status 404")
}

func (c *CLIClient) EnsureProjectFields(projectID string, fields []FieldTemplate) (map[string]ProjectField, error) {
	current, err := c.fetchProjectFields(projectID)
	if err != nil {
		return nil, err
	}
	added := false
	for _, field := range fields {
		if _, exists := current[field.Name]; exists {
			continue
		}
		if err := c.createProjectField(projectID, field); err != nil {
			return nil, err
		}
		added = true
	}
	if added || len(current) == 0 {
		return c.fetchProjectFields(projectID)
	}
	return current, nil
}

func (c *CLIClient) createProjectField(projectID string, field FieldTemplate) error {
	input := map[string]interface{}{
		"projectId": projectID,
		"name":      field.Name,
		"dataType":  field.DataType,
	}
	if strings.TrimSpace(field.Description) != "" {
		input["description"] = field.Description
	}
	if strings.EqualFold(field.DataType, "SINGLE_SELECT") && len(field.Options) > 0 {
		var options []map[string]string
		for _, opt := range field.Options {
			entry := map[string]string{"name": opt.Name}
			if strings.TrimSpace(opt.Color) != "" {
				entry["color"] = opt.Color
			}
			if strings.TrimSpace(opt.Description) != "" {
				entry["description"] = opt.Description
			}
			options = append(options, entry)
		}
		input["singleSelectOptions"] = options
	}
	variables := map[string]interface{}{"input": input}
	payload, err := json.Marshal(variables)
	if err != nil {
		return fmt.Errorf("marshal field input: %w", err)
	}
	mutation := `mutation($input:CreateProjectV2FieldInput!) {
	  createProjectV2Field(input:$input) { clientMutationId }
	}`
	args := []string{"api", "graphql", "-f", "query=" + mutation, "--raw-field", "variables=" + string(payload)}
	if _, err := c.runner.Run("gh", args...); err != nil {
		return fmt.Errorf("create project field %q: %w", field.Name, err)
	}
	return nil
}

func (c *CLIClient) fetchProjectFields(projectID string) (map[string]ProjectField, error) {
	query := `query($id:ID!) {
  node(id:$id) {
    ... on ProjectV2 {
      fields(first:100) {
        nodes {
          __typename
          ... on ProjectV2Field {
            id
            name
            dataType
          }
          ... on ProjectV2SingleSelectField {
            id
            name
            dataType
            options { id name }
          }
        }
      }
    }
  }
}`
	args := []string{"api", "graphql", "-f", "query=" + query, "-F", "id=" + projectID}
	output, err := c.runner.Run("gh", args...)
	if err != nil {
		return nil, fmt.Errorf("fetch project fields: %w", err)
	}
	var resp struct {
		Data struct {
			Node struct {
				Fields struct {
					Nodes []struct {
						Typename string `json:"__typename"`
						ID       string `json:"id"`
						Name     string `json:"name"`
						DataType string `json:"dataType"`
						Options  []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"options"`
					} `json:"nodes"`
				} `json:"fields"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("parse project fields: %w", err)
	}
	fields := make(map[string]ProjectField)
	for _, node := range resp.Data.Node.Fields.Nodes {
		pf := ProjectField{ID: node.ID, Name: node.Name, DataType: node.DataType, Options: make(map[string]ProjectFieldOption)}
		for _, opt := range node.Options {
			pf.Options[opt.Name] = ProjectFieldOption{ID: opt.ID, Name: opt.Name}
		}
		fields[pf.Name] = pf
	}
	return fields, nil
}

func (c *CLIClient) UpdateProjectItemField(projectID, itemID, fieldID string, value map[string]interface{}) error {
	input := map[string]interface{}{
		"projectId": projectID,
		"itemId":    itemID,
		"fieldId":   fieldID,
		"value":     value,
	}
	variables := map[string]interface{}{"input": input}
	payload, err := json.Marshal(variables)
	if err != nil {
		return fmt.Errorf("marshal field value: %w", err)
	}
	mutation := `mutation($input:UpdateProjectV2ItemFieldValueInput!) {
  updateProjectV2ItemFieldValue(input:$input) { projectV2Item { id } }
}`
	args := []string{"api", "graphql", "-f", "query=" + mutation, "--raw-field", "variables=" + string(payload)}
	if _, err := c.runner.Run("gh", args...); err != nil {
		return fmt.Errorf("update project field value: %w", err)
	}
	return nil
}
