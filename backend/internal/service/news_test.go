package service

import (
	"testing"
	"time"
)

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		label string
		t     time.Time
		want  string
	}{
		{"zero time", time.Time{}, "recently"},
		{"10s ago", now.Add(-10 * time.Second), "just now"},
		{"30m ago", now.Add(-30 * time.Minute), "30m ago"},
		{"3h ago", now.Add(-3 * time.Hour), "3h ago"},
		{"2d ago", now.Add(-48 * time.Hour), "2d ago"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := relativeTime(tc.t); got != tc.want {
				t.Errorf("relativeTime = %q, want %q", got, tc.want)
			}
		})
	}
}
