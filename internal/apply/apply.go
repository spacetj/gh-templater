package apply

import (
	"fmt"
	"io"
	"strconv"
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
	Org          string
	ProjectName  string
	IssuesRepo   string
	Template     string
	Sections     Sections
	DryRun       bool
	DryRunWriter io.Writer
}

// Apply executes the project bootstrapping flow.
func Apply(opts Options, client github.Client) error {
	sections := opts.Sections
	if !sections.anyEnabled() {
		sections = DefaultSections()
	}
	runner := newStepRunner(opts.DryRun, opts.DryRunWriter)

	tpl, err := template.Load(opts.Template)
	if err != nil {
		return err
	}
	projectConfig := tpl.Project
	if sections.Project && projectConfig == nil {
		fmt.Println("Template does not define a project block; skipping project creation")
		sections.Project = false
	}

	if sections.Labels {
		for _, label := range tpl.Labels {
			lbl := label
			description := fmt.Sprintf("Ensure label %q in %s", lbl.Name, opts.IssuesRepo)
			if err := runner.Run(description, func() error {
				return client.EnsureLabel(opts.IssuesRepo, lbl.Name, lbl.Color, lbl.Description)
			}); err != nil {
				return err
			}
		}
	}

	var project github.ProjectInfo
	var projectFields map[string]github.ProjectField
	if sections.Project {
		description := fmt.Sprintf("Create project %q under %s", opts.ProjectName, opts.Org)
		if err := runner.Run(description, func() error {
			var err error
			project, err = client.CreateProject(opts.Org, opts.ProjectName)
			if err != nil {
				return fmt.Errorf("create project: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}

		if err := runner.Run(fmt.Sprintf("Update README for project %q", opts.ProjectName), func() error {
			if err := client.UpdateProjectReadme(project.ID, projectConfig.Readme); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		fieldTemplates := convertFieldTemplates(projectConfig.Fields)
		if err := runner.Run(fmt.Sprintf("Ensure project fields for %q", opts.ProjectName), func() error {
			var err error
			projectFields, err = client.EnsureProjectFields(project.ID, fieldTemplates)
			if err != nil {
				return fmt.Errorf("ensure project fields: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
		if opts.DryRun {
			projectFields = simulateProjectFieldMetadata(fieldTemplates)
		}
	}

	milestoneTitles := make(map[string]struct{})
	for _, m := range tpl.Milestones {
		milestoneTitles[m.Title] = struct{}{}
		if !sections.Milestones {
			continue
		}
		milestone := m
		description := fmt.Sprintf("Create milestone %q in %s", milestone.Title, opts.IssuesRepo)
		if err := runner.Run(description, func() error {
			return client.CreateMilestone(opts.IssuesRepo, milestone.Title, milestone.Description, milestone.DueOn)
		}); err != nil {
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

			body := composeIssueBody(issue)
			issueInput := github.TemplateIssueWithResolvedMilestone{
				Title:     issue.Title,
				Body:      body,
				Labels:    issue.Labels,
				Milestone: issue.Milestone,
				Assignees: issue.Assignees,
			}

			var issueURL string
			description := fmt.Sprintf("Create issue %q in %s", issue.Title, opts.IssuesRepo)
			if err := runner.Run(description, func() error {
				var err error
				issueURL, err = client.CreateIssue(opts.IssuesRepo, issueInput)
				if err != nil {
					return err
				}
				return nil
			}); err != nil {
				return err
			}

			if sections.Project {
				var itemID string
				if err := runner.Run(fmt.Sprintf("Add issue %q to project %q", issue.Title, opts.ProjectName), func() error {
					var err error
					itemID, err = client.AddItemToProject(opts.Org, project.Number, issueURL)
					if err != nil {
						return err
					}
					return nil
				}); err != nil {
					return err
				}
				if len(issue.Fields) > 0 {
					if err := applyIssueFields(issue.Fields, opts.ProjectName, issue.Title, project.ID, itemID, projectFields, client, runner); err != nil {
						return err
					}
				}
			}
		}
	}

	if sections.Project && !opts.DryRun {
		fmt.Printf("Project created: %s\n", project.URL)
	}
	return nil
}

func convertFieldTemplates(fields []template.TemplateField) []github.FieldTemplate {
	var result []github.FieldTemplate
	for _, field := range fields {
		ft := github.FieldTemplate{
			Name:        field.Name,
			DataType:    strings.ToUpper(field.DataType),
			Description: field.Description,
		}
		for _, opt := range field.Options {
			ft.Options = append(ft.Options, github.FieldOption{Name: opt.Name, Color: opt.Color, Description: opt.Description})
		}
		result = append(result, ft)
	}
	return result
}

func simulateProjectFieldMetadata(fields []github.FieldTemplate) map[string]github.ProjectField {
	result := make(map[string]github.ProjectField, len(fields))
	for idx, field := range fields {
		options := make(map[string]github.ProjectFieldOption, len(field.Options))
		for _, opt := range field.Options {
			options[opt.Name] = github.ProjectFieldOption{ID: fmt.Sprintf("dry-run-field-%d-option-%s", idx, opt.Name), Name: opt.Name}
		}
		result[field.Name] = github.ProjectField{
			ID:       fmt.Sprintf("dry-run-field-%d", idx),
			Name:     field.Name,
			DataType: field.DataType,
			Options:  options,
		}
	}
	return result
}

func composeIssueBody(issue template.TemplateIssue) string {
	body := strings.TrimSpace(issue.Body)
	var builder strings.Builder
	if body != "" {
		builder.WriteString(body)
	}
	doc := issue.Doc
	if doc.Source != "" || doc.Link != "" || doc.Purpose != "" || len(doc.KeyActivities) > 0 {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("## Doc Context\n")
		if doc.Source != "" {
			builder.WriteString(fmt.Sprintf("Source: %s\n", doc.Source))
		}
		if doc.Link != "" {
			builder.WriteString(fmt.Sprintf("Link: %s\n", doc.Link))
		}
		if doc.Purpose != "" {
			builder.WriteString("\n### Purpose\n")
			builder.WriteString(doc.Purpose)
			builder.WriteString("\n")
		}
		if len(doc.KeyActivities) > 0 {
			builder.WriteString("\n### Key Activities\n")
			for _, activity := range doc.KeyActivities {
				builder.WriteString("- ")
				builder.WriteString(activity)
				builder.WriteString("\n")
			}
		}
	}
	return builder.String()
}

func applyIssueFields(fieldValues map[string]string, projectName, issueTitle, projectID, itemID string, projectFields map[string]github.ProjectField, client github.Client, runner stepRunner) error {
	if len(fieldValues) == 0 {
		return nil
	}
	if len(projectFields) == 0 {
		return fmt.Errorf("no project fields available to set issue fields")
	}
	for name, value := range fieldValues {
		meta, ok := projectFields[name]
		if !ok {
			return fmt.Errorf("project field %q not found", name)
		}
		payload, err := buildFieldValue(meta, value)
		if err != nil {
			return fmt.Errorf("set field %s: %w", name, err)
		}
		description := fmt.Sprintf("Set project field %q on issue %q in project %q", name, issueTitle, projectName)
		if err := runner.Run(description, func() error {
			return client.UpdateProjectItemField(projectID, itemID, meta.ID, payload)
		}); err != nil {
			return err
		}
	}
	return nil
}

func buildFieldValue(field github.ProjectField, raw string) (map[string]interface{}, error) {
	switch strings.ToUpper(field.DataType) {
	case "TEXT":
		return map[string]interface{}{"text": raw}, nil
	case "NUMBER":
		if strings.TrimSpace(raw) == "" {
			return map[string]interface{}{"number": 0.0}, nil
		}
		val, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse number %q: %w", raw, err)
		}
		return map[string]interface{}{"number": val}, nil
	case "DATE":
		return map[string]interface{}{"date": raw}, nil
	case "SINGLE_SELECT":
		option, ok := field.Options[raw]
		if !ok {
			return nil, fmt.Errorf("option %q not found", raw)
		}
		return map[string]interface{}{"singleSelectOptionId": option.ID}, nil
	default:
		return nil, fmt.Errorf("unsupported field type %s", field.DataType)
	}
}
