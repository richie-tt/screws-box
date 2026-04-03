package main

import (
	"fmt"
	"time"
)

// Shelf represents an organizer shelf with a configurable grid of containers.
type Shelf struct {
	ID        int64
	Name      string
	Rows      int
	Cols      int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Container represents a single position in the shelf grid.
type Container struct {
	ID        int64
	ShelfID   int64
	Col       int
	Row       int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Item represents a fastener or part stored in a container.
type Item struct {
	ID          int64
	ContainerID int64
	Name        string
	Description *string // nullable, per D-06
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Tag represents a lowercase-normalized label for categorizing items.
type Tag struct {
	ID        int64
	Name      string // always lowercase, per D-08
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GridData is the view model for the grid template.
type GridData struct {
	ShelfName  string
	Rows       int
	Cols       int
	ColNumbers []int // [1, 2, 3, ..., Cols] for column header iteration
	Grid       []Row
	Error      string // non-empty if shelf could not be loaded
}

// Row represents one row in the grid display.
type Row struct {
	Letter string // "A", "B", "C", ...
	Cells  []Cell
}

// Cell represents one container position.
type Cell struct {
	Coord    string // from labelFor(), e.g. "3B"
	Col      int    // 1-based
	Row      int    // 1-based
	Count    int    // number of items
	IsEmpty  bool   // true when Count == 0
	CSSClass string // "cell-light" or "cell-dark"
}

// labelFor converts a (col, row) pair to a human-readable label.
// col is the column number (1-based), row becomes a letter (1=A, 2=B, ...).
// Example: labelFor(3, 2) returns "3B".
// Per D-05: label is NEVER stored in DB, always computed by this function.
func labelFor(col, row int) string {
	return fmt.Sprintf("%d%c", col, 'A'+rune(row-1))
}
