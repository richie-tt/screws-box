package model

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
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Tag represents a lowercase-normalized label for categorizing items.
type Tag struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GridData is the view model for the grid template.
type GridData struct {
	ShelfName      string
	Rows           int
	Cols           int
	ColNumbers     []int
	Grid           []Row
	Error          string
	AuthEnabled    bool
	AuthUser       string
	AuthHasPassword bool
}

// AuthSettings holds authentication configuration.
type AuthSettings struct {
	Enabled     bool   `json:"enabled"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	HasPassword bool   `json:"has_password"`
}

// Row represents one row in the grid display.
type Row struct {
	Letter string
	Cells  []Cell
}

// Cell represents one container position.
type Cell struct {
	Coord       string
	Col         int
	Row         int
	Count       int
	IsEmpty     bool
	CSSClass    string
	ContainerID int64
}

// ItemResponse is the API-ready representation of an item.
type ItemResponse struct {
	ID             int64    `json:"id"`
	ContainerID    int64    `json:"container_id"`
	ContainerLabel string   `json:"container_label"`
	Name           string   `json:"name"`
	Description    *string  `json:"description"`
	Tags           []string `json:"tags"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

// TagResponse is the API-ready representation of a tag.
type TagResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ContainerWithItems is the response for GET /api/containers/:id/items.
type ContainerWithItems struct {
	ID        int64          `json:"id"`
	ShelfID   int64          `json:"shelf_id"`
	Col       int            `json:"col"`
	Row       int            `json:"row"`
	Label     string         `json:"label"`
	Items     []ItemResponse `json:"items"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

// ResizeRequest holds the parameters for a shelf resize operation.
type ResizeRequest struct {
	Rows int     `json:"rows"`
	Cols int     `json:"cols"`
	Name *string `json:"name,omitempty"`
}

// ResizeResult holds the outcome of a shelf resize operation.
type ResizeResult struct {
	Rows               int                 `json:"rows"`
	Cols               int                 `json:"cols"`
	Blocked            bool                `json:"blocked,omitempty"`
	Message            string              `json:"message,omitempty"`
	AffectedContainers []AffectedContainer `json:"affected,omitempty"`
	ContainersAdded    int                 `json:"containers_added,omitempty"`
	ContainersRemoved  int                 `json:"containers_removed,omitempty"`
}

// AffectedContainer describes a container that would be removed by a resize
// but still contains items.
type AffectedContainer struct {
	Label     string   `json:"label"`
	ItemCount int      `json:"item_count"`
	Items     []string `json:"items"`
}

// LabelFor converts a (col, row) pair to a human-readable label.
// col is the column number (1-based), row becomes a letter (1=A, 2=B, ...).
// Example: LabelFor(3, 2) returns "3B".
// Label is NEVER stored in DB, always computed by this function.
func LabelFor(col, row int) string {
	return fmt.Sprintf("%d%c", col, 'A'+rune(row-1))
}

// Dedup returns a new slice with duplicate strings removed, preserving order.
func Dedup(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// FormatTime formats a time.Time as RFC3339 string for API responses.
func FormatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}
