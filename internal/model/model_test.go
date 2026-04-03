package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
		assert.Equal(t, tt.want, LabelFor(tt.col, tt.row), "LabelFor(%d, %d)", tt.col, tt.row)
	}
}

func TestDedup(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, Dedup([]string{"a", "b", "a"}))
	assert.Equal(t, []string{"x"}, Dedup([]string{"x", "x", "x"}))
	assert.Empty(t, Dedup([]string{}))
	assert.Empty(t, Dedup(nil))
}

func TestFormatTime(t *testing.T) {
	ts := time.Date(2026, 4, 3, 12, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-04-03T12:30:00Z", FormatTime(ts))
}
