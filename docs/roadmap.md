# gh-templater Roadmap

## Phase 1 – Stability & UX
- Enhance CLI diagnostics and add a dry-run/preview mode.
- Improve credential guidance and scope detection for GH tokens.
- Introduce `gh templater lint` for template validation.

## Phase 2 – Templates & Extensibility
- Publish curated templates (incident response, product launch, compliance, hackathon).
- Support layered templates or inline overrides for quick tweaks.
- Allow remote templates via URLs or Git refs with caching.

## Phase 3 – Advanced Project Automation
- Expand `--sections` granularity (e.g., `project:readme`, `project:fields`).
- Detect/reconcile field renames/options with preview output.
- Optional issue linking to PRs/docs and reusable checklists.

## Phase 4 – Ecosystem Integration
- Provide reusable GitHub Action steps/workflows.
- Expose a Go library/API for third-party integrations.
- Emit structured logs/metrics or `--json` output for observability.

## Phase 5 – Governance & Collaboration
- Integrate with policy engines (OPA/Sentinel) for enforcement.
- Build a template registry with metadata, changelogs, and subscriptions.
- Support multi-project orchestration across repos/orgs with aggregated reports.
