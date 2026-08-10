// Package safety assesses the risk of buying a token before the user signs a
// swap: honeypot, buy/sell tax, and owner-power flags. The verdict is
// advisory by default and always fail-closed — when a token cannot be
// assessed the report says "unknown", never "safe".
package safety

import (
	"fmt"
	"sort"
	"strings"
)

// Level is the overall risk verdict.
type Level int

const (
	LevelLow Level = iota
	LevelMedium
	LevelHigh
)

func (l Level) String() string {
	switch l {
	case LevelHigh:
		return "HIGH"
	case LevelMedium:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// Flag is one named risk finding.
type Flag struct {
	Key      string `json:"key"`
	Desc     string `json:"desc"`
	Severity Level  `json:"severity"`
}

// Report is the outcome of a token risk check.
type Report struct {
	Score      int    `json:"score"` // 0–100, sum of flag weights
	Level      Level  `json:"level"`
	LevelName  string `json:"levelName"`
	Honeypot   bool   `json:"honeypot"`
	BuyTaxBps  int    `json:"buyTaxBps"`
	SellTaxBps int    `json:"sellTaxBps"`
	Flags      []Flag `json:"flags"`
	Source     string `json:"source"` // "goplus" | "local-sim" | ""
	// Unknown means the token could not be assessed (unsupported chain, API
	// failure, missing core data). Treat as caution, never as safe.
	Unknown bool   `json:"unknown"`
	Reason  string `json:"reason,omitempty"` // why Unknown, when set
}

// flag weights; the sum (capped at 100) is the score, thresholds give the level.
const (
	weightHigh   = 40
	weightMedium = 15
	weightLow    = 5
)

// finalize computes Score/Level/LevelName from the accumulated flags.
func (r *Report) finalize() {
	score := 0
	for _, f := range r.Flags {
		switch f.Severity {
		case LevelHigh:
			score += weightHigh
		case LevelMedium:
			score += weightMedium
		default:
			score += weightLow
		}
	}
	if score > 100 {
		score = 100
	}
	r.Score = score
	switch {
	case score >= weightHigh:
		r.Level = LevelHigh
	case score >= weightMedium:
		r.Level = LevelMedium
	default:
		r.Level = LevelLow
	}
	r.LevelName = r.Level.String()
}

// sortFlags orders flags most-severe first (stable, so source order breaks ties).
func sortFlags(fs []Flag) {
	sort.SliceStable(fs, func(i, j int) bool { return fs[i].Severity > fs[j].Severity })
}

// FormatReport renders a Report as plain terminal lines (no color — callers
// style it).
func FormatReport(r Report) string {
	var b strings.Builder
	if r.Unknown {
		fmt.Fprintf(&b, "safety: could not assess token risk")
		if r.Reason != "" {
			fmt.Fprintf(&b, " (%s)", r.Reason)
		}
		b.WriteString(" — proceed with caution")
		return b.String()
	}
	fmt.Fprintf(&b, "safety: %s risk (score %d/100, %s)", r.Level, r.Score, r.Source)
	for _, f := range r.Flags {
		marker := "•"
		if f.Severity == LevelHigh {
			marker = "⚠"
		}
		fmt.Fprintf(&b, "\n  %s %s", marker, f.Desc)
	}
	return b.String()
}
