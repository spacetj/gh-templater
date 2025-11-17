package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var userHomeDir = os.UserHomeDir

// Template describes the structure required to create a project, labels, custom fields, milestones, and issues.
type Template struct {
	Name       string              `yaml:"name"`
	Project    *TemplateProject    `yaml:"project"`
	Labels     []TemplateLabel     `yaml:"labels"`
	Milestones []TemplateMilestone `yaml:"milestones"`
	Issues     []TemplateIssue     `yaml:"issues"`
}

type TemplateProject struct {
	Readme string          `yaml:"readme"`
	Fields []TemplateField `yaml:"fields"`
}

type TemplateLabel struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Description string `yaml:"description"`
}

type TemplateField struct {
	Name        string                `yaml:"name"`
	DataType    string                `yaml:"data_type"`
	Description string                `yaml:"description"`
	Options     []TemplateFieldOption `yaml:"options"`
}

type TemplateFieldOption struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Description string `yaml:"description"`
}

// TemplateMilestone defines a milestone blueprint for the repository.
type TemplateMilestone struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	DueOn       string `yaml:"due_on"`
}

// TemplateIssue defines an issue to be created from the template.
type TemplateIssue struct {
	Title     string             `yaml:"title"`
	Body      string             `yaml:"body"`
	Labels    []string           `yaml:"labels"`
	Milestone string             `yaml:"milestone"`
	Assignees []string           `yaml:"assignees"`
	Fields    map[string]string  `yaml:"fields"`
	Doc       TemplateDocContext `yaml:"doc"`
}

type TemplateDocContext struct {
	Source        string   `yaml:"source"`
	Link          string   `yaml:"link"`
	Purpose       string   `yaml:"purpose"`
	KeyActivities []string `yaml:"key_activities"`
}

// Load reads a YAML file into a Template instance.
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

	var tpl Template
	if err := yaml.Unmarshal(content, &tpl); err != nil {
		return Template{}, fmt.Errorf("parse template yaml: %w", err)
	}

	return tpl, nil
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
