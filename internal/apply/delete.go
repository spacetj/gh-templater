package apply

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/github/gh-templater/internal/github"
	"github.com/github/gh-templater/internal/template"
)

// DeleteOptions capture inputs for removing template resources.
type DeleteOptions struct {
	Org          string
	ProjectName  string
	IssuesRepo   string
	Template     string
	Sections     Sections
	DryRun       bool
	DryRunWriter io.Writer
}

// Delete removes template-driven resources (project, milestones, issues).
func Delete(opts DeleteOptions, client github.Client) error {
	sections := opts.Sections
	if !sections.anyEnabled() {
		sections = DefaultSections()
	}
	runner := newStepRunner(opts.DryRun, opts.DryRunWriter)

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
		if err := deleteIssues(issuesRepo, tpl.Issues, client, runner); err != nil {
			return err
		}
	}
	if sections.Milestones && len(tpl.Milestones) > 0 {
		if err := deleteMilestones(issuesRepo, tpl.Milestones, client, runner); err != nil {
			return err
		}
	}
	if sections.Labels && len(tpl.Labels) > 0 {
		if err := deleteLabels(issuesRepo, tpl.Labels, client, runner); err != nil {
			return err
		}
	}
	if sections.Project {
		if err := deleteProject(org, projectName, client, runner); err != nil {
			return err
		}
	}
	return nil
}

func deleteProject(org, projectName string, client github.Client, runner stepRunner) error {
	org = strings.TrimSpace(org)
	projectName = strings.TrimSpace(projectName)
	if org == "" || projectName == "" {
		return fmt.Errorf("both --org and --project are required")
	}
	var project github.ProjectInfo
	if err := runner.Run(fmt.Sprintf("Delete project %q under %s", projectName, org), func() error {
		var err error
		project, err = client.FindProject(org, projectName)
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
		return nil
	}); err != nil {
		return err
	}
	if project.ID != "" && !runner.IsDryRun() {
		fmt.Printf("Project deleted: %s\n", project.URL)
	}
	return nil
}

func deleteMilestones(repo string, milestones []template.TemplateMilestone, client github.Client, runner stepRunner) error {
	for _, m := range milestones {
		milestone := m
		description := fmt.Sprintf("Delete milestone %q from %s", milestone.Title, repo)
		if err := runner.Run(description, func() error {
			info, err := client.FindMilestone(repo, milestone.Title)
			if err != nil {
				if errors.Is(err, github.ErrMilestoneNotFound) {
					return nil
				}
				return err
			}
			if err := client.DeleteMilestone(repo, info.Number); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func deleteIssues(repo string, issues []template.TemplateIssue, client github.Client, runner stepRunner) error {
	for _, issue := range issues {
		current := issue
		description := fmt.Sprintf("Delete issues titled %q in %s", current.Title, repo)
		if err := runner.Run(description, func() error {
			matches, err := client.FindIssues(repo, current.Title)
			if err != nil {
				return err
			}
			for _, match := range matches {
				if current.Milestone != "" && !strings.EqualFold(current.Milestone, match.Milestone) {
					continue
				}
				if len(current.Labels) > 0 && !containsAll(match.Labels, current.Labels) {
					continue
				}
				if err := client.DeleteIssue(match.ID); err != nil {
					return fmt.Errorf("delete issue %q: %w", match.Title, err)
				}
			}
			return nil
		}); err != nil {
			return err
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

func deleteLabels(repo string, labels []template.TemplateLabel, client github.Client, runner stepRunner) error {
	for _, label := range labels {
		if strings.TrimSpace(label.Name) == "" {
			continue
		}
		lbl := label
		description := fmt.Sprintf("Delete label %q from %s", lbl.Name, repo)
		if err := runner.Run(description, func() error {
			return client.DeleteLabel(repo, lbl.Name)
		}); err != nil {
			return err
		}
	}
	return nil
}
