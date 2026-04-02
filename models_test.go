package main

import "testing"

func TestLabelFor(t *testing.T) {
	tests := []struct {
		col, row int
		want     string
	}{
		{1, 1, "1A"},
		{3, 2, "3B"},
		{10, 5, "10E"},
		{1, 26, "1Z"},
		{5, 3, "5C"},
		{2, 4, "2D"},
	}
	for _, tt := range tests {
		got := labelFor(tt.col, tt.row)
		if got != tt.want {
			t.Errorf("labelFor(%d, %d) = %q, want %q", tt.col, tt.row, got, tt.want)
		}
	}
}
