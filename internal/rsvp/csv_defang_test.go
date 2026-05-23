package rsvp

import "testing"

func TestDefangCSVCell(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"benign text", "Alice", "Alice"},
		{"equals formula", "=cmd|'/c calc'!A0", "'=cmd|'/c calc'!A0"},
		{"plus formula", "+1+1", "'+1+1"},
		{"minus formula", "-2+3", "'-2+3"},
		{"at formula", "@SUM(A1:A10)", "'@SUM(A1:A10)"},
		{"tab prefix", "\t=evil", "'\t=evil"},
		{"cr prefix", "\rdanger", "'\rdanger"},
		{"phone leading plus", "+14155551234", "'+14155551234"},
		{"email is benign", "alice@example.com", "alice@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DefangCSVCell(tc.in)
			if got != tc.want {
				t.Fatalf("DefangCSVCell(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
