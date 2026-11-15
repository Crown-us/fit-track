package domain

import (
	"math"
	"time"
)

// DateLayout is the calendar-date format used across the API and database.
const DateLayout = "2006-01-02"

const (
	// DefaultPageLimit is used when the client does not send a limit.
	DefaultPageLimit = 20
	// MaxPageLimit bounds the page size so one request cannot dump a table.
	MaxPageLimit = 100

	oneDecimalFactor = 10
)

// Page selects a window of a list.
type Page struct {
	Limit  int
	Offset int
}

// DateRange filters by calendar date; an empty bound means unbounded.
type DateRange struct {
	From string
	To   string
}

// ParseDate parses a YYYY-MM-DD calendar date.
func ParseDate(value string) (time.Time, error) {
	return time.Parse(DateLayout, value)
}

// IsDate reports whether value is a valid YYYY-MM-DD calendar date.
func IsDate(value string) bool {
	_, err := ParseDate(value)
	return err == nil
}

// Round1 rounds to one decimal place, which is the precision food tables and
// calorie estimates can honestly claim.
func Round1(value float64) float64 {
	return math.Round(value*oneDecimalFactor) / oneDecimalFactor
}
