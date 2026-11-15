// Package coach turns a weekly summary into short, actionable guidance.
package coach

import (
	"context"

	"fittrack/internal/summary"
)

// Disclaimer accompanies every piece of advice.
const Disclaimer = "Saran ini bersifat panduan umum, bukan nasihat medis. " +
	"Konsultasikan dengan tenaga kesehatan untuk kebutuhan khusus."

// Message is one piece of guidance, tagged with the rule that produced it.
type Message struct {
	Rule string `json:"rule"`
	Text string `json:"text"`
}

// Advice is the coach's response for one week.
type Advice struct {
	WeekStart  string    `json:"week_start"`
	WeekEnd    string    `json:"week_end"`
	Messages   []Message `json:"messages"`
	Disclaimer string    `json:"disclaimer"`
}

// Coach produces advice from a weekly summary. It is an interface so an
// LLM-backed coach can replace the rule-based one without touching callers;
// ctx and the error exist for implementations that call a remote model.
type Coach interface {
	Advise(ctx context.Context, week summary.Weekly) (Advice, error)
}

// RuleBased is a deterministic Coach built from pure rule functions.
type RuleBased struct{}

// NewRuleBased returns the rule-based coach.
func NewRuleBased() RuleBased {
	return RuleBased{}
}

// Advise evaluates every rule against week.
func (RuleBased) Advise(_ context.Context, week summary.Weekly) (Advice, error) {
	return Advice{
		WeekStart:  week.WeekStart,
		WeekEnd:    week.WeekEnd,
		Messages:   Evaluate(week),
		Disclaimer: Disclaimer,
	}, nil
}

// Evaluate runs all rules in a fixed order and returns the messages of those
// that fire, or a single encouragement when none does.
func Evaluate(week summary.Weekly) []Message {
	messages := make([]Message, 0, len(rules))
	for _, evaluate := range rules {
		if message, hasFired := evaluate(week); hasFired {
			messages = append(messages, message)
		}
	}
	if len(messages) == 0 {
		return []Message{{Rule: RuleOnTrack, Text: "Minggu ini berjalan sesuai rencana. Pertahankan ya!"}}
	}
	return messages
}
