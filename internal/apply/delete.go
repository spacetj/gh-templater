package apply

import (
	"errors"
	"fmt"
	"strings"

	"github.com/github/gh-templater/internal/github"
	"github.com/github/gh-templater/internal/template"
)

// DeleteOptions capture inputs for removing template resources.
type DeleteOptions struct {
	Org         string
	ProjectName string
	IssuesRepo  string
	Template    string
	Sections    Sections
}

// Delete removes template-driven resources (project, milestones, issues).
func Delete(opts DeleteOptions, client github.Client) error {
	sections := opts.Sections
	if !sections.anyEnabled() {
		sections = DefaultSections()
	}

	org := strings.TrimSpace(opts.Org)
	projectName := strings.TrimSpace(opts.ProjectName)
	issuesRepo := strings.TrimSpace(opts.IssuesRepo)
	templatePath := strings.TrimSpace(opts.Template)

	var tpl template.Template
	var err error
	if templatePath != "" {
		tpl, err = template.Load(templatePath)
		if err != nil {
			return err
		}
	}

	if (sections.Milestones || sections.Issues || sections.Labels) && templatePath == "" {
		return fmt.Errorf("--template is required when deleting labels, milestones, or issues")
	}
	if (sections.Milestones || sections.Issues || sections.Labels) && issuesRepo == "" {
		return fmt.Errorf("--issues-repo is required when deleting labels, milestones, or issues")
	}

	if sections.Issues && len(tpl.Issues) > 0 {
		if err := deleteIssues(issuesRepo, tpl.Issues, client); err != nil {
			return err
		}
	}
	if sections.Milestones && len(tpl.Milestones) > 0 {
		if err := deleteMilestones(issuesRepo, tpl.Milestones, client); err != nil {
			return err
		}
	}
	if sections.Labels && len(tpl.Labels) > 0 {
		if err := deleteLabels(issuesRepo, tpl.Labels, client); err != nil {
			return err
		}
	}
	if sections.Project {
		if err := deleteProject(org, projectName, client); err != nil {
			return err
		}
	}
	return nil
}

func deleteProject(org, projectName string, client github.Client) error {
	org = strings.TrimSpace(org)
	projectName = strings.TrimSpace(projectName)
	if org == "" || projectName == "" {
		return fmt.Errorf("both --org and --project are required")
	}
	project, err := client.FindProject(org, projectName)
	if err != nil {
		if errors.Is(err, github.ErrProjectNotFound) {
			fmt.Printf("Project %q not found under %s; skipping delete\n", projectName, org)
			return nil
		}
		return err
	}
	if project.ID == "" {
		fmt.Printf("Project %q not found under %s; skipping delete\n", projectName, org)
		return nil
	}
	if err := client.DeleteProject(project.ID); err != nil {
		return err
	}
	fmt.Printf("Project deleted: %s\n", project.URL)
	return nil
}

func deleteMilestones(repo string, milestones []template.TemplateMilestone, client github.Client) error {
	for _, m := range milestones {
		info, err := client.FindMilestone(repo, m.Title)
		if err != nil {
			if errors.Is(err, github.ErrMilestoneNotFound) {
				continue
			}
			return err
		}
		if err := client.DeleteMilestone(repo, info.Number); err != nil {
			return err
		}
	}
	return nil
}

func deleteIssues(repo string, issues []template.TemplateIssue, client github.Client) error {
	for _, issue := range issues {
		matches, err := client.FindIssues(repo, issue.Title)
		if err != nil {
			return err
		}
		for _, match := range matches {
			if issue.Milestone != "" && !strings.EqualFold(issue.Milestone, match.Milestone) {
				continue
			}
			if len(issue.Labels) > 0 && !containsAll(match.Labels, issue.Labels) {
				continue
			}
			if err := client.DeleteIssue(match.ID); err != nil {
				return fmt.Errorf("delete issue %q: %w", match.Title, err)
			}
		}
	}
	return nil
}

func containsAll(haystack []string, needles []string) bool {
	if len(needles) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(haystack))
	for _, val := range haystack {
		set[strings.ToLower(val)] = struct{}{}
	}
	for _, needle := range needles {
		if _, ok := set[strings.ToLower(needle)]; !ok {
			return false
		}
	}
	return true
}

func deleteLabels(repo string, labels []template.TemplateLabel, client github.Client) error {
	for _, label := range labels {
		if strings.TrimSpace(label.Name) == "" {
			continue
		}
		if err := client.DeleteLabel(repo, label.Name); err != nil {
			return err
		}
	}
	return nil
}
