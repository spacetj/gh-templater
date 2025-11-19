# Stop Copy-Pasting Kickoff Tickets: Meet `gh-templater`

Week three of a new initiative inevitably feels like déjà vu. You open an old issue, change a couple of names, forget to reword the acceptance criteria, then ping three teams to fill in the missing context. Tejas Chopra’s CNPR series nails that pain: when you don’t invest in repeatable workflows, you pay the tax every single sprint. `gh-templater` is the antidote—codifying the “how we start work here” so every project begins from the same, proven scaffold.

## Life Without Templates Is All Drag

- **Blank-page paralysis:** Teams keep private copies of “the good ticket” because an empty issue feels like falling into space. By the time you tweak headers and rewrite the checklist, momentum is gone.
- **Context decay:** Institutional knowledge lives in side conversations. When a new PM or engineer joins, they rebuild rituals by guessing, not by following a shared playbook.
- **Quality roulette:** Two squads can ship wildly different artifacts for the same milestone. Reviews bog down on formatting instead of substance, and leaders lose the ability to compare apples to apples.
- **Launch-day scramble:** Copy/paste workflows shed steps—labels, milestones, project fields—so launch checklists are incomplete and teams scramble to reconcile the gaps.

You shouldn’t have to be the human macro for your org. That is what machines are for.

## Enter `gh-templater`

`gh-templater` is a GitHub CLI extension that turns a declarative YAML file into an entire project skeleton: Projects v2 board, README, labels, milestones, and the issues that populate them. Instead of everyone improvising PowerPoint-to-issue pipelines, you run one command:

```bash
gh templater apply \
  --org your-org \
  --project "Platform Readiness" \
  --issues-repo your-org/roadmap \
  --template templates/backend-grpc.yaml
```

Behind that single CLI call, the tool creates or updates the destination project, syncs labels, generates milestones, files issues, and wires everything together. Because the template lives in git, changing the “definition of done” happens once—in the template—and every future instantiation inherits it automatically.

## Codify the Task Anatomy Once

Templates can be as light or opinionated as your team requires:

```yaml
name: Reliability Sprint
project:
  readme: |
    # Reliability Sprint
    Tracking our availability goals.
  fields:
    - name: Spec State
      data_type: SINGLE_SELECT
      options:
        - name: Draft
labels:
  - name: spec
    color: 9B59B6
milestones:
  - title: Hardening
issues:
  - title: Audit error budget
    body: Collect SLI data for the past 90 days
    milestone: Hardening
    fields:
      Spec State: Draft
```

Need only labels and issues? Use `--sections labels,issues` and the CLI runs just those steps. Want to tear everything down after a dry run? `gh templater delete` mirrors the same sections so you can keep your org tidy.

## Reproducible Tasks Beat Heroic Copy/Paste

Think about the ripple effects once your team stops rebuilding the same scaffolding every sprint:

- **Faster starts:** Kickoffs happen in minutes, not days. The command becomes part of the runbook: fork template, run CLI, assign owners.
- **Reproducible QA:** Because issues, milestones, and project fields are identical every time, metrics dashboards and retros stay comparable across launches. You can finally answer “how long does hardening usually take?”
- **Cross-team fluency:** Product, design, infra, and support all see the same sections for risk, rollback, metrics, and comms. There is no translation tax for “what goes in the release checklist?”
- **Versioned rituals:** Templates live next to code. If a retro reveals a missing step, modify the YAML and open a PR; the discussion, review, and history all live in one place.
- **Onboarding superpowers:** New teammates don’t memorize lore. They run the templater, read the generated README, and fill in structured fields that highlight the sharp edges of your process.

## Beyond Tickets: Reproducible Sprints

`gh-templater` shines when you treat Sprints, launches, and campaigns as reproducible tasks. Define the canonical story anatomy, standard QA protocol, or customer escalation flow, then bake it into the template. Every time the command runs, you get:

1. A populated Projects v2 board with custom fields for status, risk, or launch tier.
2. Labels that sync across repos so filtering “launch-critical” actually works.
3. Milestones aligned with your operating rhythm (shape, build, harden, or whatever fits your team).
4. Issues pre-filled with checklists, metrics gates, rollback plans, and owners.

This is how you make work reproducible. Not by hoping everyone remembers the ritual, but by encoding it in a tool the whole org can trust.

## Bonus Meme

**Caption:** “What my PM thinks my job is vs. what I actually do.”

- *Left panel:* Engineer triumphantly shipping code—label it “Building features.”
- *Right panel:* Same engineer frantically copy/pasting Markdown headers, checkboxes, and labels—caption “Rewriting the same ticket for the 47th time.”
- *Punchline banner:* “`gh-templater`: because creativity shouldn’t be wasted on formatting.”

## Try It on Your Next Kickoff

Identify the canonical project, sprint, or launch in your org. Capture the rituals that make it successful (fields, checklists, communication plan) in a template file, commit it to the repo, and run `gh templater apply`. Suddenly every new initiative starts with consistency, context, and less stress. Instead of reliving week-three déjà vu, your team can focus on the work that actually moves the metric.
