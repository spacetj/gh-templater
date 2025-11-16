package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/github/gh-templater/internal/runner"
)

// Client describes the operations required to create projects, milestones, and issues.
type Client interface {
	CreateProject(owner, title string) (ProjectInfo, error)
	UpdateProjectReadme(projectID, readme string) error
	CreateMilestone(repo string, milestoneTitle, description, dueOn string) error
	CreateIssue(repo string, issue TemplateIssueWithResolvedMilestone) (string, error)
	AddItemToProject(owner string, projectNumber int, itemURL string) error
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
	orgID, err := c.lookupOrganizationID(owner)
	if err != nil {
		return ProjectInfo{}, err
	}

	query := `mutation($ownerId:ID!, $title:String!) { createProjectV2(input:{ownerId:$ownerId, title:$title}) { projectV2 { id number url } } }`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-F", "ownerId="+orgID, "-F", "title="+title)
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
func (c *CLIClient) AddItemToProject(owner string, projectNumber int, itemURL string) error {
	args := []string{"project", "item-add", "--owner", owner, "--project-number", fmt.Sprintf("%d", projectNumber), "--url", itemURL}
	if _, err := c.runner.Run("gh", args...); err != nil {
		return fmt.Errorf("add item to project: %w", err)
	}
	return nil
}

func (c *CLIClient) lookupOrganizationID(owner string) (string, error) {
	query := `query($login:String!){ organization(login:$login) { id } }`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-F", "login="+owner)
	if err != nil {
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
		return "", fmt.Errorf("organization %s not found", owner)
	}

	return parsed.Data.Organization.ID, nil
}
