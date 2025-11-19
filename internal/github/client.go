package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
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
	DeleteLabel(repo, name string) error
	EnsureProjectFields(projectID string, fields []FieldTemplate) (map[string]ProjectField, error)
	UpdateProjectItemField(projectID, itemID, fieldID string, value map[string]interface{}) error
	FindProject(owner, title string) (ProjectInfo, error)
	DeleteProject(projectID string) error
	FindMilestone(repo, title string) (MilestoneInfo, error)
	DeleteMilestone(repo string, number int) error
	FindIssues(repo, title string) ([]IssueInfo, error)
	DeleteIssue(issueID string) error
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

type MilestoneInfo struct {
	Number int
	Title  string
}

type IssueInfo struct {
	ID        string
	Number    int
	Title     string
	URL       string
	Milestone string
	Labels    []string
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

func (c *CLIClient) FindProject(owner, title string) (ProjectInfo, error) {
	project, err := c.findProjectForOwnerType("organization", owner, title)
	if err == nil {
		return project, nil
	}
	if errors.Is(err, ErrOrganizationNotFound) {
		return c.findProjectForOwnerType("user", owner, title)
	}
	return ProjectInfo{}, err
}

func (c *CLIClient) findProjectForOwnerType(ownerType, owner, title string) (ProjectInfo, error) {
	query := fmt.Sprintf(`query($login:String!, $search:String!) {
  %s(login:$login) {
    projectsV2(first: 20, query: $search) {
      nodes { id title number url }
    }
  }
}`, ownerType)
	args := []string{"api", "graphql", "-f", "query=" + query, "-F", "login=" + owner, "-F", "search=" + title}
	output, err := c.runner.Run("gh", args...)
	if err != nil {
		switch ownerType {
		case "organization":
			if strings.Contains(err.Error(), "Could not resolve to an Organization") {
				return ProjectInfo{}, fmt.Errorf("%w: %s", ErrOrganizationNotFound, owner)
			}
		case "user":
			if strings.Contains(err.Error(), "Could not resolve to a User") {
				return ProjectInfo{}, fmt.Errorf("%w: %s", ErrUserNotFound, owner)
			}
		}
		return ProjectInfo{}, fmt.Errorf("query projects: %w", err)
	}
	type projectNode struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	var resp struct {
		Data struct {
			Organization *struct {
				Projects struct {
					Nodes []projectNode `json:"nodes"`
				} `json:"projectsV2"`
			} `json:"organization"`
			User *struct {
				Projects struct {
					Nodes []projectNode `json:"nodes"`
				} `json:"projectsV2"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return ProjectInfo{}, fmt.Errorf("parse project query: %w", err)
	}
	var nodes []projectNode
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
	for _, node := range nodes {
		if node.Title == title {
			return ProjectInfo{ID: node.ID, Number: node.Number, URL: node.URL}, nil
		}
	}
	return ProjectInfo{}, fmt.Errorf("%w: %s", ErrProjectNotFound, title)
}

func (c *CLIClient) DeleteProject(projectID string) error {
	mutation := `mutation($projectId:ID!) {
  deleteProjectV2(input:{projectId:$projectId}) {
    clientMutationId
  }
}`
	if _, err := c.runGraphQL(mutation, map[string]interface{}{"projectId": projectID}); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

func (c *CLIClient) FindMilestone(repo, title string) (MilestoneInfo, error) {
	path := fmt.Sprintf("repos/%s/milestones?state=all&per_page=100", repo)
	output, err := c.runner.Run("gh", "api", path)
	if err != nil {
		return MilestoneInfo{}, fmt.Errorf("list milestones: %w", err)
	}
	var milestones []struct {
		Title  string `json:"title"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal([]byte(output), &milestones); err != nil {
		return MilestoneInfo{}, fmt.Errorf("parse milestones: %w", err)
	}
	for _, m := range milestones {
		if m.Title == title {
			return MilestoneInfo{Title: m.Title, Number: m.Number}, nil
		}
	}
	return MilestoneInfo{}, fmt.Errorf("%w: %s", ErrMilestoneNotFound, title)
}

func (c *CLIClient) DeleteMilestone(repo string, number int) error {
	path := fmt.Sprintf("repos/%s/milestones/%d", repo, number)
	if _, err := c.runner.Run("gh", "api", path, "--method", "DELETE"); err != nil {
		return fmt.Errorf("delete milestone %d: %w", number, err)
	}
	return nil
}

func (c *CLIClient) FindIssues(repo, title string) ([]IssueInfo, error) {
	query := `query($search:String!) {
  search(query:$search, type:ISSUE, first:20) {
    nodes {
      __typename
      ... on Issue {
        id
        number
        title
        url
        repository { nameWithOwner }
        milestone { title }
        labels(first:20) { nodes { name } }
      }
    }
  }
}`
	search := fmt.Sprintf("repo:%s in:title %q", repo, title)
	output, err := c.runGraphQL(query, map[string]interface{}{"search": search})
	if err != nil {
		return nil, fmt.Errorf("search issues: %w", err)
	}
	var resp struct {
		Data struct {
			Search struct {
				Nodes []struct {
					ID         string `json:"id"`
					Title      string `json:"title"`
					Number     int    `json:"number"`
					URL        string `json:"url"`
					Repository struct {
						NameWithOwner string `json:"nameWithOwner"`
					} `json:"repository"`
					Milestone *struct {
						Title string `json:"title"`
					} `json:"milestone"`
					Labels struct {
						Nodes []struct {
							Name string `json:"name"`
						} `json:"nodes"`
					} `json:"labels"`
				} `json:"nodes"`
			} `json:"search"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("parse issue search: %w", err)
	}
	var issues []IssueInfo
	for _, node := range resp.Data.Search.Nodes {
		if !strings.EqualFold(node.Repository.NameWithOwner, repo) {
			continue
		}
		var labels []string
		for _, l := range node.Labels.Nodes {
			labels = append(labels, l.Name)
		}
		milestoneTitle := ""
		if node.Milestone != nil {
			milestoneTitle = node.Milestone.Title
		}
		issues = append(issues, IssueInfo{
			ID:        node.ID,
			Number:    node.Number,
			Title:     node.Title,
			URL:       node.URL,
			Milestone: milestoneTitle,
			Labels:    labels,
		})
	}
	return issues, nil
}

func (c *CLIClient) DeleteIssue(issueID string) error {
	mutation := `mutation($issueId:ID!) {
  deleteIssue(input:{issueId:$issueId}) {
    clientMutationId
  }
}`
	if _, err := c.runGraphQL(mutation, map[string]interface{}{"issueId": issueID}); err != nil {
		return fmt.Errorf("delete issue: %w", err)
	}
	return nil
}

var (
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrUserNotFound         = errors.New("user not found")
	ErrProjectNotFound      = errors.New("project not found")
	ErrMilestoneNotFound    = errors.New("milestone not found")
)

func (c *CLIClient) lookupOrganizationID(owner string) (string, error) {
	query := `query($login:String!){ organization(login:$login) { id } }`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-F", "login="+owner)
	if err != nil {
		if strings.Contains(err.Error(), "Could not resolve to an Organization") {
			return "", fmt.Errorf("%w: %s", ErrOrganizationNotFound, owner)
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
		return "", fmt.Errorf("%w: %s", ErrOrganizationNotFound, owner)
	}

	return parsed.Data.Organization.ID, nil
}

func (c *CLIClient) lookupOwnerID(owner string) (string, error) {
	id, err := c.lookupOrganizationID(owner)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, ErrOrganizationNotFound) {
		return "", err
	}
	return c.lookupUserID(owner)
}

func (c *CLIClient) lookupUserID(owner string) (string, error) {
	query := `query($login:String!){ user(login:$login) { id } }`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-F", "login="+owner)
	if err != nil {
		if strings.Contains(err.Error(), "Could not resolve to a User") {
			return "", fmt.Errorf("%w: %s", ErrUserNotFound, owner)
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
		return "", fmt.Errorf("%w: %s", ErrUserNotFound, owner)
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

func (c *CLIClient) DeleteLabel(repo, name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	path := fmt.Sprintf("repos/%s/labels/%s", repo, url.PathEscape(name))
	if _, err := c.runner.Run("gh", "api", path, "--method", "DELETE"); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete label %q: %w", name, err)
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
	modified := len(current) == 0
	for _, field := range fields {
		existing, exists := current[field.Name]
		if !exists {
			if err := c.createProjectField(projectID, field); err != nil {
				return nil, err
			}
			modified = true
			continue
		}
		if !strings.EqualFold(existing.DataType, field.DataType) {
			return nil, fmt.Errorf("project field %q already exists with type %s (expected %s)", field.Name, existing.DataType, field.DataType)
		}
		if strings.EqualFold(field.DataType, "SINGLE_SELECT") && len(field.Options) > 0 {
			addedOptions, err := c.ensureProjectFieldOptions(existing, field.Options)
			if err != nil {
				return nil, err
			}
			if addedOptions {
				modified = true
			}
		}
	}
	if modified {
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
	mutation := `mutation($input:CreateProjectV2FieldInput!) {
  createProjectV2Field(input:$input) { clientMutationId }
}`
	variables := map[string]interface{}{"input": input}
	if _, err := c.runGraphQL(mutation, variables); err != nil {
		return fmt.Errorf("create project field %q: %w", field.Name, err)
	}
	return nil
}

func (c *CLIClient) ensureProjectFieldOptions(field ProjectField, options []FieldOption) (bool, error) {
	if len(options) == 0 {
		return false, nil
	}
	added := false
	for _, opt := range options {
		if _, exists := field.Options[opt.Name]; exists {
			continue
		}
		if err := c.createProjectFieldOption(field.ID, opt); err != nil {
			return false, err
		}
		added = true
	}
	return added, nil
}

func (c *CLIClient) createProjectFieldOption(fieldID string, option FieldOption) error {
	input := map[string]interface{}{
		"fieldId": fieldID,
		"name":    option.Name,
	}
	if strings.TrimSpace(option.Color) != "" {
		input["color"] = option.Color
	}
	mutation := `mutation($input:CreateProjectV2FieldOptionInput!) {
  createProjectV2FieldOption(input:$input) { projectV2FieldOption { id } }
}`
	if _, err := c.runGraphQL(mutation, map[string]interface{}{"input": input}); err != nil {
		return fmt.Errorf("create project field option %q: %w", option.Name, err)
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
	mutation := `mutation($input:UpdateProjectV2ItemFieldValueInput!) {
  updateProjectV2ItemFieldValue(input:$input) { projectV2Item { id } }
}`
	if _, err := c.runGraphQL(mutation, map[string]interface{}{"input": input}); err != nil {
		return fmt.Errorf("update project field value: %w", err)
	}
	return nil
}

func (c *CLIClient) runGraphQL(query string, variables map[string]interface{}) (string, error) {
	payload := map[string]interface{}{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp("", "gh-graphql-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		if cerr := file.Close(); cerr != nil {
			return "", fmt.Errorf("write graphql payload: %v (close error: %w)", err, cerr)
		}
		return "", fmt.Errorf("write graphql payload: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	args := []string{"api", "graphql", "--input", file.Name()}
	return c.runner.Run("gh", args...)
}
