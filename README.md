# gh-templater

> A GitHub CLI extension that turns YAML project templates into fully-provisioned GitHub Projects with milestones and issues.

## Why
Modern teams repeat the same bootstrap steps for every new initiative. `gh-templater` lets you codify those steps as YAML so
anyone can spin up a ready-to-track project with a single command—no copy/paste required.

## Quickstart

1. [Install GitHub CLI](https://docs.github.com/en/github-cli/github-cli/quickstart) and ensure you are authenticated (`gh auth login`).
2. Install the extension:
   ```bash
   gh extension install <your-org>/gh-templater
   ```
3. Pick a template (see `templates/` for examples) and apply it:
   ```bash
   gh templater apply \
     --org your-org \
     --project "Platform Readiness" \
     --issues-repo your-org/roadmap \
     --template templates/backend-grpc.yaml
   ```

The command will:
- Create a GitHub Project (Projects v2) under the org you specify.
- Apply the template README content to the project.
- Create milestones and issues in the target repository.
- Add each issue to the new project automatically.

## Template format
Templates are YAML files that describe the project README, milestones, and issues. A minimal example:

```yaml
name: Reliability Sprint
readme: |
  # Reliability Sprint
  Tracking our availability goals.
milestones:
  - title: Hardening
    description: Improve error budgets
issues:
  - title: Audit error budget
    body: Collect SLI data for the past 90 days
    labels: [reliability]
    milestone: Hardening
```

See [`docs/templates.md`](docs/templates.md) for the full schema and extension-specific tips.

## Commands

- `gh templater apply`: Apply a YAML template to an organization and issues repository.

Run `gh templater apply --help` to see the complete flag list.

## Development

This extension follows the [GitHub CLI extension](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions) model.

### Requirements
- Go 1.21+
- (Optional) [`golangci-lint`](https://golangci-lint.run/usage/install/) for richer linting

### Useful scripts

The repository includes a `Makefile` for a fast feedback loop:

```bash
make fmt   # gofmt all Go files
make lint  # run golangci-lint if available, otherwise go vet
make test  # run unit tests
make build # compile the gh-templater binary
```

### Local execution

Clone the repo and use `gh` to run the local extension without installing globally:

```bash
gh extension exec ./gh-templater templater apply --org my-org --project "My Project" --issues-repo my-org/roadmap --template templates/backend-grpc.yaml
```

## Inspiration
- [`gh-projects`](https://github.com/github/gh-projects) for CLI ergonomics
- [`gk-cli`](https://github.com/gitkraken/gk-cli) for documentation tone and structure

## Contributing
- Open an issue or PR with your proposal.
- Add tests for new behaviors.
- Keep templates and docs in sync so users can move from README to execution quickly.
