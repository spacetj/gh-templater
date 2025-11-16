package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTemplate(t *testing.T) {
	path := t.TempDir() + "/template.yaml"
	content := `name: Sample
readme: README content
milestones:
  - title: Kickoff
    description: Start the project
issues:
  - title: First issue
    body: Do something
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	tpl, err := Load(path)
	if err != nil {
		t.Fatalf("load template: %v", err)
	}

	if tpl.Name != "Sample" {
		t.Fatalf("expected name 'Sample', got %q", tpl.Name)
	}

	if len(tpl.Milestones) != 1 {
		t.Fatalf("expected 1 milestone, got %d", len(tpl.Milestones))
	}

	if len(tpl.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(tpl.Issues))
	}
}

func TestLoadTemplateParsesLabels(t *testing.T) {
	path := t.TempDir() + "/template.yaml"
	content := `labels:
  - name: spec
    color: 9B59B6
    description: Spec-driven work
  - name: bug
    color: "#ff0000"
`
	writeTemplate(t, path, content)

	tpl, err := Load(path)
	if err != nil {
		t.Fatalf("load template: %v", err)
	}

	if len(tpl.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %+v", len(tpl.Labels), tpl.Labels)
	}
	if tpl.Labels[0].Name != "spec" || tpl.Labels[0].Color != "9B59B6" {
		t.Fatalf("unexpected label parsed: %+v", tpl.Labels[0])
	}
	if tpl.Labels[1].Color != "#ff0000" {
		t.Fatalf("expected raw color preserved, got %s", tpl.Labels[1].Color)
	}
}

func TestLoadTemplateExpandsHomeShortcut(t *testing.T) {
	originalHomeFunc := userHomeDir
	t.Cleanup(func() { userHomeDir = originalHomeFunc })

	fakeHome := t.TempDir()
	userHomeDir = func() (string, error) {
		return fakeHome, nil
	}

	path := filepath.Join(fakeHome, "template.yaml")
	content := `name: Sample`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	if _, err := Load("~/template.yaml"); err != nil {
		t.Fatalf("load template with ~ prefix: %v", err)
	}
}

func TestLoadTemplateRequiresPath(t *testing.T) {
	_, err := Load(" ")
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected error for empty path, got %v", err)
	}
}

func writeTemplate(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
}
