# Ticket & Issue Workflow

## Issue Tracking Tool

**GitHub Issues** — No external tool integration (Linear, Jira, etc.). All issues and project management is native to GitHub.

## Workflow States & Practices

GitHub Issues standard states:
- **Open** — New issues or work in progress
- **Closed** — Resolved or abandoned

No custom workflow states are configured (would require GitHub Projects or external tool).

## Project Structure

Phoenix uses structured issue templates and conventions:
- `.github/ISSUE_TEMPLATE/` — Multiple issue types (feature request, bug report, etc.)
- `CODEOWNERS` — Code ownership rules for auto-assignment and review routing

## Contribution & Release Process

### Feature/Issue Handling
1. Open issue in GitHub
2. Create feature branch or draft PR
3. CI runs (linting, tests, security checks)
4. Code review (Claude Code or human)
5. Merge to main

### Automated Workflows
- **release-please** — Tracks version history in `.release-please-manifest.json`; auto-creates PRs for version bumps and changelog
- **Semantic versioning** — Major.Minor.Patch across all packages

### Issue-Driven Features
Phoenix includes automation for issue handling:
- **claude-implement-issue.yml** — Claude-powered issue implementation workflow
- **claude-code-review.yml** — Automated code review via Claude
- **collect-customer-issues.yaml** — Gather feedback from issues

## Communication Channels

- **Slack** — Community support (public invite for Arize Phoenix channel)
- **GitHub Discussions** — Alternative to issues (implied but not explicitly configured)
- **Documentation** — ReadTheDocs at arize.com/docs/phoenix

## Summary

**No production ticket system.** Development uses GitHub Issues exclusively; no Linear, Jira, or Asana integration. Emphasis on automated workflows (release-please, Claude AI agents) for issue routing and implementation.

