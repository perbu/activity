package scheduler

import (
	"testing"
	"time"

	"github.com/perbu/activity/internal/config"
)

func TestNextFireTime(t *testing.T) {
	loc := time.UTC

	// Default config: Monday at 02:42
	cfg := config.DefaultConfig()
	sched := &Scheduler{cfg: cfg}

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "Saturday afternoon -> next Monday 02:42",
			now:  time.Date(2026, 2, 28, 15, 0, 0, 0, loc), // Saturday
			want: time.Date(2026, 3, 2, 2, 42, 0, 0, loc),  // Monday
		},
		{
			name: "Monday 02:41 -> same Monday 02:42",
			now:  time.Date(2026, 3, 2, 2, 41, 0, 0, loc),
			want: time.Date(2026, 3, 2, 2, 42, 0, 0, loc),
		},
		{
			name: "Monday 02:42 exactly -> next Monday 02:42",
			now:  time.Date(2026, 3, 2, 2, 42, 0, 0, loc),
			want: time.Date(2026, 3, 9, 2, 42, 0, 0, loc),
		},
		{
			name: "Monday 02:43 -> next Monday 02:42",
			now:  time.Date(2026, 3, 2, 2, 43, 0, 0, loc),
			want: time.Date(2026, 3, 9, 2, 42, 0, 0, loc),
		},
		{
			name: "Sunday 23:59 -> next day Monday 02:42",
			now:  time.Date(2026, 3, 1, 23, 59, 0, 0, loc), // Sunday
			want: time.Date(2026, 3, 2, 2, 42, 0, 0, loc),  // Monday
		},
		{
			name: "Wednesday midday -> next Monday 02:42",
			now:  time.Date(2026, 3, 4, 12, 0, 0, 0, loc), // Wednesday
			want: time.Date(2026, 3, 9, 2, 42, 0, 0, loc),  // Monday
		},
		{
			name: "year boundary - last Monday of December after 02:42",
			now:  time.Date(2026, 12, 28, 10, 0, 0, 0, loc), // Monday
			want: time.Date(2027, 1, 4, 2, 42, 0, 0, loc),   // next Monday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sched.nextFireTime(tt.now)
			if !got.Equal(tt.want) {
				t.Errorf("nextFireTime(%v) = %v, want %v", tt.now, got, tt.want)
			}
			if got.Weekday() != time.Monday {
				t.Errorf("nextFireTime returned %v, not a Monday", got.Weekday())
			}
			if !got.After(tt.now) {
				t.Errorf("nextFireTime(%v) = %v, not strictly after now", tt.now, got)
			}
		})
	}
}

func TestNextFireTimeCustomDay(t *testing.T) {
	loc := time.UTC
	cfg := config.DefaultConfig()
	cfg.Schedule.Day = "wednesday"
	cfg.Schedule.Hour = 9
	cfg.Schedule.Minute = 0
	sched := &Scheduler{cfg: cfg}

	// Monday -> next Wednesday at 09:00
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, loc) // Monday
	got := sched.nextFireTime(now)
	want := time.Date(2026, 3, 4, 9, 0, 0, 0, loc) // Wednesday
	if !got.Equal(want) {
		t.Errorf("nextFireTime(%v) = %v, want %v", now, got, want)
	}
}
