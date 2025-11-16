package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var userHomeDir = os.UserHomeDir

// Template describes the structure required to create a project, milestones, and issues.
type Template struct {
	Name       string
	Readme     string
	Milestones []TemplateMilestone
	Issues     []TemplateIssue
}

// TemplateMilestone defines a milestone blueprint for the repository.
type TemplateMilestone struct {
	Title       string
	Description string
	DueOn       string
}

// TemplateIssue defines an issue to be created from the template.
type TemplateIssue struct {
	Title     string
	Body      string
	Labels    []string
	Milestone string
	Assignees []string
}

// Load reads a YAML file into a Template instance.
// The parser is a focused implementation that supports the subset of YAML used by the built-in templates
// (maps, lists, and block scalars).

func Load(path string) (Template, error) {
	absPath, err := resolveTemplatePath(path)
	if err != nil {
		return Template{}, err
	}
	dir := filepath.Dir(absPath)
	file := filepath.Base(absPath)

	content, err := fs.ReadFile(os.DirFS(dir), file)
	if err != nil {
		return Template{}, fmt.Errorf("read template: %w", err)
	}

	parser := yamlSubset{lines: strings.Split(string(content), "\n")}
	tpl, err := parser.parse()
	if err != nil {
		return Template{}, fmt.Errorf("parse template yaml: %w", err)
	}

	return tpl, nil
}

type yamlSubset struct {
	lines []string
	pos   int
}

func (y *yamlSubset) next() bool {
	y.pos++
	return y.pos < len(y.lines)
}

func (y *yamlSubset) current() string {
	if y.pos >= len(y.lines) {
		return ""
	}
	return y.lines[y.pos]
}

func (y *yamlSubset) parse() (Template, error) {
	var tpl Template
	for y.pos = 0; y.pos < len(y.lines); y.pos++ {
		line := strings.TrimSpace(y.current())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "name:"):
			tpl.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		case strings.HasPrefix(line, "readme:"):
			readme, err := y.parseBlock()
			if err != nil {
				return Template{}, err
			}
			tpl.Readme = readme
		case strings.HasPrefix(line, "milestones:"):
			items, err := y.parseList()
			if err != nil {
				return Template{}, err
			}
			tpl.Milestones = items
		case strings.HasPrefix(line, "issues:"):
			issues, err := y.parseIssueList()
			if err != nil {
				return Template{}, err
			}
			tpl.Issues = issues
		}
	}
	return tpl, nil
}

func (y *yamlSubset) parseBlock() (string, error) {
	line := strings.TrimSpace(y.current())
	if !strings.HasSuffix(line, "|") {
		return strings.TrimSpace(strings.TrimPrefix(line, "readme:")), nil
	}

	var builder []string
	for y.next() {
		raw := y.current()
		if strings.TrimSpace(raw) == "" && len(builder) == 0 {
			continue
		}
		if len(raw) > 0 && (raw[0] != ' ' && raw[0] != '\t') {
			y.pos--
			break
		}
		builder = append(builder, strings.TrimLeft(raw, " \t"))
	}

	return strings.Join(builder, "\n"), nil
}

func (y *yamlSubset) parseList() ([]TemplateMilestone, error) {
	var milestones []TemplateMilestone
	for y.next() {
		line := y.current()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "  -") && !strings.HasPrefix(line, "-") {
			y.pos--
			break
		}
		item := TemplateMilestone{}
		for {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				parts := strings.SplitN(strings.TrimPrefix(trimmed, "- "), ":", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) == "title" {
					item.Title = strings.TrimSpace(parts[1])
				}
			} else if strings.HasPrefix(trimmed, "title:") {
				item.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "title:"))
			} else if strings.HasPrefix(trimmed, "description:") {
				item.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			} else if strings.HasPrefix(trimmed, "due_on:") {
				item.DueOn = strings.TrimSpace(strings.TrimPrefix(trimmed, "due_on:"))
			}

			if !y.next() {
				break
			}
			line = y.current()
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "  ") {
				y.pos--
				break
			}
		}
		milestones = append(milestones, item)
	}
	return milestones, nil
}

func (y *yamlSubset) parseIssueList() ([]TemplateIssue, error) {
	var issues []TemplateIssue
	for y.next() {
		line := y.current()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "  -") && !strings.HasPrefix(line, "-") {
			y.pos--
			break
		}
		issue := TemplateIssue{}
		for {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "- "):
				parts := strings.SplitN(strings.TrimPrefix(trimmed, "- "), ":", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) == "title" {
					issue.Title = strings.TrimSpace(parts[1])
				}
			case strings.HasPrefix(trimmed, "title:"):
				issue.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "title:"))
			case strings.HasPrefix(trimmed, "body:"):
				body, err := y.parseBodyBlock(trimmed)
				if err != nil {
					return nil, err
				}
				issue.Body = body
			case strings.HasPrefix(trimmed, "labels:"):
				labels := strings.TrimSpace(strings.TrimPrefix(trimmed, "labels:"))
				labels = strings.Trim(labels, "[]")
				if labels != "" {
					for _, l := range strings.Split(labels, ",") {
						cleaned := strings.TrimSpace(strings.Trim(l, "[]"))
						if cleaned != "" {
							issue.Labels = append(issue.Labels, cleaned)
						}
					}
				}
			case strings.HasPrefix(trimmed, "milestone:"):
				issue.Milestone = strings.TrimSpace(strings.TrimPrefix(trimmed, "milestone:"))
			case strings.HasPrefix(trimmed, "assignees:"):
				assignees := strings.TrimSpace(strings.TrimPrefix(trimmed, "assignees:"))
				assignees = strings.Trim(assignees, "[]")
				if assignees != "" {
					for _, a := range strings.Split(assignees, ",") {
						cleaned := strings.TrimSpace(strings.Trim(a, "[]"))
						if cleaned != "" {
							issue.Assignees = append(issue.Assignees, cleaned)
						}
					}
				}
			}

			if !y.next() {
				break
			}
			line = y.current()
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "  ") {
				y.pos--
				break
			}
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (y *yamlSubset) parseBodyBlock(current string) (string, error) {
	if !strings.HasSuffix(current, "|") {
		return strings.TrimSpace(strings.TrimPrefix(current, "body:")), nil
	}
	var builder []string
	for y.next() {
		raw := y.current()
		if len(raw) > 0 && (raw[0] != ' ' && raw[0] != '\t') {
			y.pos--
			break
		}
		builder = append(builder, strings.TrimLeft(raw, " \t"))
	}
	return strings.Join(builder, "\n"), nil
}

func resolveTemplatePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("template path is required")
	}

	expanded, err := expandHome(trimmed)
	if err != nil {
		return "", err
	}

	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve template path: %w", err)
	}
	return abs, nil
}

func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	if path == "~" {
		return home, nil
	}

	switch path[1] {
	case '/', '\\':
		return filepath.Join(home, path[2:]), nil
	default:
		return path, nil
	}
}
