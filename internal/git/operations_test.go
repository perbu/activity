package git

import (
	"testing"
	"time"
)

func TestParseISOWeek(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantYear int
		wantWeek int
		wantErr  bool
	}{
		// Valid formats
		{"valid week 02", "2026-W02", 2026, 2, false},
		{"valid week 01", "2026-W01", 2026, 1, false},
		{"valid week 52", "2025-W52", 2025, 52, false},
		{"valid week 53", "2020-W53", 2020, 53, false},
		{"single digit week", "2026-W05", 2026, 5, false},
		{"week 10", "2026-W10", 2026, 10, false},

		// Invalid formats
		{"missing W prefix", "2026-02", 0, 0, true},
		{"lowercase w", "2026-w02", 0, 0, true},
		{"no dash", "2026W02", 0, 0, true},
		{"empty string", "", 0, 0, true},
		{"garbage", "invalid", 0, 0, true},
		{"only year", "2026", 0, 0, true},
		{"date format", "2026-01-15", 0, 0, true},

		// Week out of range
		{"week 0", "2026-W00", 0, 0, true},
		{"week 54", "2026-W54", 0, 0, true},
		{"week 99", "2026-W99", 0, 0, true},
		{"negative week", "2026-W-1", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotYear, gotWeek, err := ParseISOWeek(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseISOWeek(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotYear != tt.wantYear {
					t.Errorf("ParseISOWeek(%q) year = %v, want %v", tt.input, gotYear, tt.wantYear)
				}
				if gotWeek != tt.wantWeek {
					t.Errorf("ParseISOWeek(%q) week = %v, want %v", tt.input, gotWeek, tt.wantWeek)
				}
			}
		})
	}
}

func TestFormatISOWeek(t *testing.T) {
	tests := []struct {
		name string
		year int
		week int
		want string
	}{
		{"normal case", 2026, 2, "2026-W02"},
		{"single digit week", 2026, 5, "2026-W05"},
		{"week 1", 2026, 1, "2026-W01"},
		{"week 10", 2026, 10, "2026-W10"},
		{"week 52", 2025, 52, "2025-W52"},
		{"week 53", 2020, 53, "2020-W53"},
		{"year 2000", 2000, 15, "2000-W15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatISOWeek(tt.year, tt.week)
			if got != tt.want {
				t.Errorf("FormatISOWeek(%d, %d) = %q, want %q", tt.year, tt.week, got, tt.want)
			}
		})
	}
}

func TestFormatParseRoundtrip(t *testing.T) {
	tests := []struct {
		year int
		week int
	}{
		{2026, 1},
		{2026, 2},
		{2026, 10},
		{2026, 52},
		{2020, 53},
	}

	for _, tt := range tests {
		t.Run(FormatISOWeek(tt.year, tt.week), func(t *testing.T) {
			formatted := FormatISOWeek(tt.year, tt.week)
			gotYear, gotWeek, err := ParseISOWeek(formatted)
			if err != nil {
				t.Errorf("roundtrip failed: FormatISOWeek(%d, %d) = %q, ParseISOWeek returned error: %v",
					tt.year, tt.week, formatted, err)
				return
			}
			if gotYear != tt.year || gotWeek != tt.week {
				t.Errorf("roundtrip failed: input (%d, %d) -> %q -> (%d, %d)",
					tt.year, tt.week, formatted, gotYear, gotWeek)
			}
		})
	}
}

func TestISOWeekBounds(t *testing.T) {
	tests := []struct {
		name      string
		year      int
		week      int
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "2026 week 1",
			year:      2026,
			week:      1,
			wantStart: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 1, 4, 23, 59, 59, 0, time.UTC),
		},
		{
			name:      "2026 week 2",
			year:      2026,
			week:      2,
			wantStart: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 1, 11, 23, 59, 59, 0, time.UTC),
		},
		{
			name:      "2025 week 52",
			year:      2025,
			week:      52,
			wantStart: time.Date(2025, 12, 22, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 12, 28, 23, 59, 59, 0, time.UTC),
		},
		{
			// 2020 has 53 weeks (ISO 8601)
			name:      "2020 week 53",
			year:      2020,
			week:      53,
			wantStart: time.Date(2020, 12, 28, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2021, 1, 3, 23, 59, 59, 0, time.UTC),
		},
		{
			name:      "middle of year 2026 week 26",
			year:      2026,
			week:      26,
			wantStart: time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 6, 28, 23, 59, 59, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := ISOWeekBounds(tt.year, tt.week)

			if !gotStart.Equal(tt.wantStart) {
				t.Errorf("ISOWeekBounds(%d, %d) start = %v, want %v",
					tt.year, tt.week, gotStart, tt.wantStart)
			}
			if !gotEnd.Equal(tt.wantEnd) {
				t.Errorf("ISOWeekBounds(%d, %d) end = %v, want %v",
					tt.year, tt.week, gotEnd, tt.wantEnd)
			}

			// Verify start is a Monday
			if gotStart.Weekday() != time.Monday {
				t.Errorf("ISOWeekBounds(%d, %d) start weekday = %v, want Monday",
					tt.year, tt.week, gotStart.Weekday())
			}

			// Verify end is a Sunday
			if gotEnd.Weekday() != time.Sunday {
				t.Errorf("ISOWeekBounds(%d, %d) end weekday = %v, want Sunday",
					tt.year, tt.week, gotEnd.Weekday())
			}

			// Verify times: start should be 00:00:00, end should be 23:59:59
			if gotStart.Hour() != 0 || gotStart.Minute() != 0 || gotStart.Second() != 0 {
				t.Errorf("ISOWeekBounds(%d, %d) start time = %02d:%02d:%02d, want 00:00:00",
					tt.year, tt.week, gotStart.Hour(), gotStart.Minute(), gotStart.Second())
			}
			if gotEnd.Hour() != 23 || gotEnd.Minute() != 59 || gotEnd.Second() != 59 {
				t.Errorf("ISOWeekBounds(%d, %d) end time = %02d:%02d:%02d, want 23:59:59",
					tt.year, tt.week, gotEnd.Hour(), gotEnd.Minute(), gotEnd.Second())
			}
		})
	}
}

func TestISOWeekBoundsConsistency(t *testing.T) {
	// Verify that dates within ISOWeekBounds have the correct ISO week
	tests := []struct {
		year int
		week int
	}{
		{2026, 1},
		{2026, 2},
		{2026, 26},
		{2026, 52},
		{2020, 53},
	}

	for _, tt := range tests {
		t.Run(FormatISOWeek(tt.year, tt.week), func(t *testing.T) {
			start, end := ISOWeekBounds(tt.year, tt.week)

			// Check start date has correct ISO week
			startYear, startWeek := start.ISOWeek()
			if startYear != tt.year || startWeek != tt.week {
				t.Errorf("start date %v has ISO week %d-W%02d, want %d-W%02d",
					start, startYear, startWeek, tt.year, tt.week)
			}

			// Check end date has correct ISO week
			endYear, endWeek := end.ISOWeek()
			if endYear != tt.year || endWeek != tt.week {
				t.Errorf("end date %v has ISO week %d-W%02d, want %d-W%02d",
					end, endYear, endWeek, tt.year, tt.week)
			}
		})
	}
}

func TestWeeksInRange(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  [][2]int
	}{
		{
			name:  "single day within one week",
			start: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), // Tuesday of W02
			end:   time.Date(2026, 1, 6, 23, 59, 59, 0, time.UTC),
			want:  [][2]int{{2026, 2}},
		},
		{
			name:  "full single week",
			start: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),  // Monday of W02
			end:   time.Date(2026, 1, 11, 23, 59, 59, 0, time.UTC), // Sunday of W02
			want:  [][2]int{{2026, 2}},
		},
		{
			name:  "two consecutive weeks",
			start: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),  // Monday W02
			end:   time.Date(2026, 1, 18, 23, 59, 59, 0, time.UTC), // Sunday W03
			want:  [][2]int{{2026, 2}, {2026, 3}},
		},
		{
			name:  "three weeks",
			start: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),  // W02
			end:   time.Date(2026, 1, 25, 23, 59, 59, 0, time.UTC), // W04
			want:  [][2]int{{2026, 2}, {2026, 3}, {2026, 4}},
		},
		{
			name:  "year boundary 2025/2026",
			start: time.Date(2025, 12, 28, 0, 0, 0, 0, time.UTC), // Sunday of 2025-W52
			end:   time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),   // Monday of 2026-W02
			want:  [][2]int{{2025, 52}, {2026, 1}, {2026, 2}},
		},
		{
			name:  "within week 1 of 2026 (spans year boundary)",
			start: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC), // Monday of 2026-W01
			end:   time.Date(2026, 1, 4, 23, 59, 59, 0, time.UTC), // Sunday of 2026-W01
			want:  [][2]int{{2026, 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeeksInRange(tt.start, tt.end)

			if len(got) != len(tt.want) {
				t.Errorf("WeeksInRange() returned %d weeks, want %d: got %v, want %v",
					len(got), len(tt.want), got, tt.want)
				return
			}

			for i, pair := range got {
				if pair[0] != tt.want[i][0] || pair[1] != tt.want[i][1] {
					t.Errorf("WeeksInRange() week %d = [%d, %d], want [%d, %d]",
						i, pair[0], pair[1], tt.want[i][0], tt.want[i][1])
				}
			}
		})
	}
}

func TestWeeksInRangeDeduplication(t *testing.T) {
	// Multiple days in the same week should only return that week once
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)  // Monday W02
	end := time.Date(2026, 1, 11, 23, 59, 59, 0, time.UTC) // Sunday W02

	weeks := WeeksInRange(start, end)

	if len(weeks) != 1 {
		t.Errorf("expected 1 unique week, got %d: %v", len(weeks), weeks)
	}

	if weeks[0][0] != 2026 || weeks[0][1] != 2 {
		t.Errorf("expected [2026, 2], got %v", weeks[0])
	}
}

func TestWeeksInRangeEmpty(t *testing.T) {
	// End before start should return empty (or handle gracefully)
	start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	weeks := WeeksInRange(start, end)

	if len(weeks) != 0 {
		t.Errorf("expected empty result when end < start, got %v", weeks)
	}
}
