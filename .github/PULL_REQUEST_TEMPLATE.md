<!--
Thanks for this. A few notes so nothing wastes your weekend:

- I review PRs on weekends and aim to decide within a week.
- I will not force-push your branch, and I will not close your PR and
  reimplement it myself. Your commits keep your authorship.
- Key handling, anything that moves funds in a new way, and large refactors
  want an issue first — see CONTRIBUTING.md.

Delete any part of this template that does not apply.
-->

## What this changes

<!-- One or two sentences. If it fixes an issue, "Fixes #123" here. -->

## Why

<!-- The problem it solves. If the README or a doc said something the code did
     not do, say so — that is a bug and I want to know. -->

## How I verified it

<!-- Be concrete: the command you ran, the output you got. For anything that
     quotes or signs, say which chain and whether it was testnet. -->

## Checklist

- [ ] `make gate` passes locally (lint + the coverage floor; `make setup` first if you have not)
- [ ] `go test ./...` passes
- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org/) — the prefix decides the next version number
- [ ] Commits are signed off with `git commit -s` ([DCO](https://developercertificate.org/); there is no CLA)
- [ ] No `//nolint` added — if a linter is wrong about the code, say so below and we will fix the config instead
- [ ] Golden files, if any changed, were regenerated with `go test ./internal/cli/ -update` **and the diff was read like code** — a golden diff is a user-visible change

## If you used an AI assistant

A one-line note is appreciated but not required. The one condition is the same
one I hold myself to: **you ran it and you understand it** — you can answer
"why does this line exist?" for every line here. See the AI section of
CONTRIBUTING.md.

<!-- Optional: which assistant, and what you changed by hand afterwards. -->
