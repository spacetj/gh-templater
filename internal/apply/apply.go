package apply

import (
	"fmt"
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
	projectConfig := tpl.Project
	if sections.Project && projectConfig == nil {
		fmt.Println("Template does not define a project block; skipping project creation")
		sections.Project = false
	}

	if sections.Labels {
		for _, label := range tpl.Labels {
			if err := client.EnsureLabel(opts.IssuesRepo, label.Name, label.Color, label.Description); err != nil {
				return err
			}
		}
	}

	var project github.ProjectInfo
	var projectFields map[string]github.ProjectField
	if sections.Project {
		project, err = client.CreateProject(opts.Org, opts.ProjectName)
		if err != nil {
			return fmt.Errorf("create project: %w", err)
		}

		if err := client.UpdateProjectReadme(project.ID, projectConfig.Readme); err != nil {
			return err
		}

		fieldTemplates := convertFieldTemplates(projectConfig.Fields)
		projectFields, err = client.EnsureProjectFields(project.ID, fieldTemplates)
		if err != nil {
			return fmt.Errorf("ensure project fields: %w", err)
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

			body := composeIssueBody(issue)
			issueInput := github.TemplateIssueWithResolvedMilestone{
				Title:     issue.Title,
				Body:      body,
				Labels:    issue.Labels,
				Milestone: issue.Milestone,
				Assignees: issue.Assignees,
			}

			url, err := client.CreateIssue(opts.IssuesRepo, issueInput)
			if err != nil {
				return err
			}

			if sections.Project {
				itemID, err := client.AddItemToProject(opts.Org, project.Number, url)
				if err != nil {
					return err
				}
				if len(issue.Fields) > 0 {
					if err := applyIssueFields(issue.Fields, project.ID, itemID, projectFields, client); err != nil {
						return err
					}
				}
			}
		}
	}

	if sections.Project {
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

func applyIssueFields(fieldValues map[string]string, projectID, itemID string, projectFields map[string]github.ProjectField, client github.Client) error {
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
		if err := client.UpdateProjectItemField(projectID, itemID, meta.ID, payload); err != nil {
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
