#!/usr/bin/env bash
set -euo pipefail
# Quality gate: total line coverage must be >= 70%, measured without the TUI
# packages. Per CLAUDE.md, View()/layout code is never tested, which makes
# internal/tui structurally unable to reach the bar — its Update logic is
# still tested (the packages run in `go test ./...`), it just doesn't count
# toward the denominator. Everything else does.
cd "$(dirname "$0")/.."

LAZYSWAP_TEST=1 go test -race -covermode=atomic -coverprofile=coverage.out ./...
grep -v '/internal/tui/' coverage.out > coverage.gate.out

echo
echo "── least-covered functions (gate scope) ──"
go tool cover -func=coverage.gate.out | sort -t$'\t' -k3 -n | head -20 || true
echo

total=$(go tool cover -func=coverage.gate.out | awk '/^total:/ {gsub("%","",$3); print $3}')
awk -v t="$total" 'BEGIN {
  if (t+0 < 70) { print "FAIL: coverage " t "% < 70% (ex-TUI)"; exit 1 }
  print "OK: coverage " t "% (>= 70%, ex-TUI)"
}'
