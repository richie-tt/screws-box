package model

// ExportData is the top-level structure for data export/import.
type ExportData struct {
	Version    int         `json:"version"`
	ExportedAt string      `json:"exported_at"`
	Shelf      ExportShelf `json:"shelf"`
}

// ExportShelf represents the shelf in an export.
type ExportShelf struct {
	Name       string            `json:"name"`
	Rows       int               `json:"rows"`
	Cols       int               `json:"cols"`
	Containers []ExportContainer `json:"containers"`
}

// ExportContainer represents a container in an export (no DB IDs).
type ExportContainer struct {
	Col   int          `json:"col"`
	Row   int          `json:"row"`
	Label string       `json:"label"`
	Items []ExportItem `json:"items"`
}

// ExportItem represents an item in an export (no DB IDs).
type ExportItem struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags"`
}
