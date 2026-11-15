package summary

import (
	"testing"
	"time"

	"github.com/Crown-us/fit-track/internal/domain"
)

func TestTodayFollowsClockTimezone(t *testing.T) {
	jakarta := time.FixedZone("WIB", 7*60*60)
	// 17:30 UTC on Sunday the 15th is already 00:30 on Monday the 16th in WIB.
	instant := time.Date(2026, time.November, 15, 17, 30, 0, 0, time.UTC)
	tests := []struct {
		name          string
		location      *time.Location
		wantToday     string
		wantWeekStart string
	}{
		{name: "utc", location: time.UTC, wantToday: "2026-11-15", wantWeekStart: "2026-11-09"},
		{name: "jakarta", location: jakarta, wantToday: "2026-11-16", wantWeekStart: "2026-11-16"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(Sources{}, func() time.Time { return instant.In(tt.location) })
			if got := service.Today().Format(domain.DateLayout); got != tt.wantToday {
				t.Errorf("Today() = %s, want %s", got, tt.wantToday)
			}
			if got := service.CurrentWeekStart().Format(domain.DateLayout); got != tt.wantWeekStart {
				t.Errorf("CurrentWeekStart() = %s, want %s", got, tt.wantWeekStart)
			}
		})
	}
}

func TestCurrentWeekStartIsMonday(t *testing.T) {
	// 2026-11-09 is a Monday; every day of that week maps back to it.
	for offset := range DaysInWeek {
		now := time.Date(2026, time.November, 9+offset, 10, 0, 0, 0, time.UTC)
		service := NewService(Sources{}, func() time.Time { return now })
		if got := service.CurrentWeekStart().Format(domain.DateLayout); got != "2026-11-09" {
			t.Errorf("CurrentWeekStart() on %s = %s, want 2026-11-09", now.Format(domain.DateLayout), got)
		}
	}
}
