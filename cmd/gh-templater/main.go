package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/github/gh-templater/internal/apply"
	"github.com/github/gh-templater/internal/github"
	"github.com/github/gh-templater/internal/runner"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "apply":
		applyCmd(os.Args[2:])
	case "delete":
		deleteCmd(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
	}
}

func applyCmd(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	org := fs.String("org", "", "Organization that will own the project")
	project := fs.String("project", "", "Name for the new GitHub project")
	issuesRepo := fs.String("issues-repo", "", "owner/repo that will receive milestones and issues")
	templatePath := fs.String("template", "", "Path to the YAML template to apply")
	sectionsFlag := fs.String("sections", "all", "Comma-separated sections to apply (project,labels,milestones,issues or 'all')")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *org == "" || *project == "" || *issuesRepo == "" || *templatePath == "" {
		fmt.Fprintln(os.Stderr, "all of --org, --project, --issues-repo, and --template are required")
		fs.Usage()
		os.Exit(1)
	}

	sections, err := apply.ParseSections(*sectionsFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client := github.NewCLIClient(runner.ExecRunner{})
	opts := apply.Options{Org: *org, ProjectName: *project, IssuesRepo: *issuesRepo, Template: *templatePath, Sections: sections}
	if err := apply.Apply(opts, client); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func deleteCmd(args []string) {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	org := fs.String("org", "", "Organization or user that owns the project")
	project := fs.String("project", "", "Name of the GitHub project to delete")
	issuesRepo := fs.String("issues-repo", "", "owner/repo containing issues and milestones to cleanup")
	templatePath := fs.String("template", "", "Path to the YAML template describing issues and milestones to delete")
	sectionsFlag := fs.String("sections", "all", "Comma-separated sections to delete (project,labels,milestones,issues or 'all')")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sections, err := apply.ParseSections(*sectionsFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if sections.Project && (*org == "" || *project == "") {
		fmt.Fprintln(os.Stderr, "both --org and --project are required when deleting a project")
		fs.Usage()
		os.Exit(1)
	}
	client := github.NewCLIClient(runner.ExecRunner{})
	opts := apply.DeleteOptions{
		Org:         *org,
		ProjectName: *project,
		IssuesRepo:  *issuesRepo,
		Template:    *templatePath,
		Sections:    sections,
	}
	if err := apply.Delete(opts, client); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "gh-templater is a GitHub CLI extension.\n")
	fmt.Fprintf(os.Stderr, "Usage:\n  gh templater apply --org <org> --project <name> --issues-repo <owner/repo> --template <path>\n  gh templater delete --org <org> --project <name> --issues-repo <owner/repo> --template <path>\n")
	os.Exit(1)
}
