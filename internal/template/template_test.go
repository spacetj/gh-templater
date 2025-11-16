package template

import (
	"os"
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
