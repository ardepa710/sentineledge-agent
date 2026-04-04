package updater

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		current   string
		want      bool
	}{
		{"newer date", "v2026.03.15-1", "v2026.03.10-1", true},
		{"same version", "v2026.03.10-1", "v2026.03.10-1", false},
		{"older date", "v2026.03.10-1", "v2026.03.15-1", false},
		{"newer build same date", "v2026.03.10-2", "v2026.03.10-1", true},
		{"older build same date", "v2026.03.10-1", "v2026.03.10-2", false},
		{"numeric build: 10 > 9", "v2026.03.10-10", "v2026.03.10-9", true},
		{"newer year", "v2027.01.01-1", "v2026.12.31-1", true},
		{"no v prefix candidate", "2026.03.15-1", "v2026.03.10-1", true},
		{"no v prefix current", "v2026.03.15-1", "2026.03.10-1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNewer(tt.candidate, tt.current)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.candidate, tt.current, got, tt.want)
			}
		})
	}
}
