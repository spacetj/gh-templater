package apply

import (
	"fmt"

	"github.com/github/gh-templater/internal/github"
	"github.com/github/gh-templater/internal/template"
)

// Options describe inputs provided via CLI flags.
type Options struct {
	Org         string
	ProjectName string
	IssuesRepo  string
	Template    string
}

// Apply executes the project bootstrapping flow.
func Apply(opts Options, client github.Client) error {
	tpl, err := template.Load(opts.Template)
	if err != nil {
		return err
	}

	project, err := client.CreateProject(opts.Org, opts.ProjectName)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}

	if err := client.UpdateProjectReadme(project.ID, tpl.Readme); err != nil {
		return err
	}

	milestoneTitles := make(map[string]struct{})
	for _, m := range tpl.Milestones {
		if err := client.CreateMilestone(opts.IssuesRepo, m.Title, m.Description, m.DueOn); err != nil {
			return err
		}
		milestoneTitles[m.Title] = struct{}{}
	}

	for _, issue := range tpl.Issues {
		if issue.Milestone != "" {
			if _, ok := milestoneTitles[issue.Milestone]; !ok {
				return fmt.Errorf("issue %q references unknown milestone %q", issue.Title, issue.Milestone)
			}
		}

		issueInput := github.TemplateIssueWithResolvedMilestone{
			Title:     issue.Title,
			Body:      issue.Body,
			Labels:    issue.Labels,
			Milestone: issue.Milestone,
			Assignees: issue.Assignees,
		}

		url, err := client.CreateIssue(opts.IssuesRepo, issueInput)
		if err != nil {
			return err
		}

		if err := client.AddItemToProject(opts.Org, project.Number, url); err != nil {
			return err
		}
	}

	fmt.Printf("Project created: %s\n", project.URL)
	return nil
}
