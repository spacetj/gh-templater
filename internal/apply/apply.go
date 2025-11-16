package apply

import (
	"fmt"
	"strings"

	"github.com/github/gh-templater/internal/github"
	"github.com/github/gh-templater/internal/template"
)

// Sections describe which template parts should be applied.
type Sections struct {
	Project    bool
	Labels     bool
	Milestones bool
	Issues     bool
}

// DefaultSections returns a Sections struct with every part enabled.
func DefaultSections() Sections {
	return Sections{Project: true, Labels: true, Milestones: true, Issues: true}
}

// ParseSections accepts a comma-delimited list (e.g., "labels,issues") and enables only
// the requested sections. The word "all" enables everything.
func ParseSections(input string) (Sections, error) {
	if strings.TrimSpace(input) == "" || strings.EqualFold(strings.TrimSpace(input), "all") {
		return DefaultSections(), nil
	}
	sections := Sections{}
	for _, token := range strings.Split(input, ",") {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" {
			continue
		}
		switch token {
		case "project":
			sections.Project = true
		case "labels":
			sections.Labels = true
		case "milestones":
			sections.Milestones = true
		case "issues":
			sections.Issues = true
		case "all":
			return DefaultSections(), nil
		default:
			return Sections{}, fmt.Errorf("unknown section %q", token)
		}
	}
	if !sections.anyEnabled() {
		return Sections{}, fmt.Errorf("no valid sections specified")
	}
	return sections, nil
}

func (s Sections) anyEnabled() bool {
	return s.Project || s.Labels || s.Milestones || s.Issues
}

// Options describe inputs provided via CLI flags.
type Options struct {
	Org         string
	ProjectName string
	IssuesRepo  string
	Template    string
	Sections    Sections
}

// Apply executes the project bootstrapping flow.
func Apply(opts Options, client github.Client) error {
	sections := opts.Sections
	if !sections.anyEnabled() {
		sections = DefaultSections()
	}

	tpl, err := template.Load(opts.Template)
	if err != nil {
		return err
	}

	if sections.Labels {
		for _, label := range tpl.Labels {
			if err := client.EnsureLabel(opts.IssuesRepo, label.Name, label.Color, label.Description); err != nil {
				return err
			}
		}
	}

	var project github.ProjectInfo
	if sections.Project {
		project, err = client.CreateProject(opts.Org, opts.ProjectName)
		if err != nil {
			return fmt.Errorf("create project: %w", err)
		}

		if err := client.UpdateProjectReadme(project.ID, tpl.Readme); err != nil {
			return err
		}
	}

	milestoneTitles := make(map[string]struct{})
	for _, m := range tpl.Milestones {
		milestoneTitles[m.Title] = struct{}{}
		if !sections.Milestones {
			continue
		}
		if err := client.CreateMilestone(opts.IssuesRepo, m.Title, m.Description, m.DueOn); err != nil {
			return err
		}
	}

	if sections.Issues {
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

			if sections.Project {
				if err := client.AddItemToProject(opts.Org, project.Number, url); err != nil {
					return err
				}
			}
		}
	}

	if sections.Project {
		fmt.Printf("Project created: %s\n", project.URL)
	}
	return nil
}
