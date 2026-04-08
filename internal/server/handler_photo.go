package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"screws-box/internal/imaging"
	"screws-box/internal/model"
	"screws-box/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// requirePhotosEnabled is middleware that returns 404 when the photos feature is disabled.
// The feature appears non-existent rather than forbidden (per D-16).
func (srv *Server) requirePhotosEnabled(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enabled, _ := srv.store.IsPhotosEnabled(r.Context())
		if !enabled {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extFromContentType maps a detected content type to a file extension.
// Returns empty string for unsupported types.
func extFromContentType(ct string) string {
	switch {
	case strings.HasPrefix(ct, "image/jpeg"):
		return ".jpg"
	case strings.HasPrefix(ct, "image/png"):
		return ".png"
	default:
		return ""
	}
}

// handlePhotoUpload accepts a multipart upload, validates the file, processes it
// through the imaging pipeline, stores both full and thumbnail, and creates a DB record.
func (srv *Server) handlePhotoUpload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit request body to 10MB + 1KB overhead for multipart headers.
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20+1024)

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			// Check for MaxBytesError (file too large).
			var maxErr *http.MaxBytesError
			if ok := matchMaxBytesError(err, &maxErr); ok {
				writeError(w, http.StatusRequestEntityTooLarge, "File exceeds the 10 MB limit.")
				return
			}
			writeError(w, http.StatusBadRequest, "Invalid multipart form.")
			return
		}

		file, header, err := r.FormFile("photo")
		if err != nil {
			writeError(w, http.StatusBadRequest, "Missing photo field.")
			return
		}
		defer file.Close()

		// Read first 512 bytes for magic byte detection.
		head := make([]byte, 512)
		n, err := io.ReadAtLeast(file, head, 1)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Cannot read file.")
			return
		}
		head = head[:n]

		// Validate content type via magic bytes (not trusting Content-Type header).
		detectedType := http.DetectContentType(head)
		ext := extFromContentType(detectedType)
		if ext == "" {
			writeError(w, http.StatusBadRequest, "Only JPEG and PNG files are accepted.")
			return
		}

		// Reconstruct reader with the head bytes prepended.
		combinedReader := io.MultiReader(bytes.NewReader(head), file)

		// Parse optional item_id.
		var itemID *int64
		if idStr := r.FormValue("item_id"); idStr != "" {
			parsed, parseErr := strconv.ParseInt(idStr, 10, 64)
			if parseErr == nil {
				itemID = &parsed
			}
		}

		// Parse optional crop_mode (default "fit").
		cropMode := r.FormValue("crop_mode")
		if cropMode != "fit" && cropMode != "crop" {
			cropMode = "fit"
		}

		// Get thumbnail size from settings.
		thumbSize, err := srv.store.GetThumbnailSize(r.Context())
		if err != nil || thumbSize <= 0 {
			thumbSize = 200
		}

		// Generate UUID for this photo.
		photoUUID := uuid.New().String()

		// Process image through the imaging pipeline (decode, EXIF strip, thumbnail).
		result, err := imaging.ProcessUpload(combinedReader, ext, thumbSize, thumbSize, cropMode)
		if err != nil {
			slog.Error("imaging pipeline failed", "err", err)
			writeError(w, http.StatusInternalServerError, "Failed to process image.")
			return
		}

		// Store full-size image.
		if err := srv.photos.Store(r.Context(), photoUUID, ext, result.FullData); err != nil {
			slog.Error("store full photo failed", "err", err)
			writeError(w, http.StatusInternalServerError, "Failed to store photo.")
			return
		}

		// Store thumbnail.
		if err := srv.photos.StoreThumbnail(r.Context(), photoUUID, ext, result.ThumbData); err != nil {
			slog.Error("store thumbnail failed", "err", err)
			writeError(w, http.StatusInternalServerError, "Failed to store thumbnail.")
			return
		}

		// Determine content type for DB record.
		contentType := detectedType
		if idx := strings.Index(contentType, ";"); idx != -1 {
			contentType = contentType[:idx]
		}

		// Create DB record.
		photo := model.Photo{
			UUID:             photoUUID,
			ItemID:           itemID,
			OriginalFilename: header.Filename,
			ContentType:      contentType,
			Ext:              ext,
			FileSize:         int64(result.FullData.Len()),
			CropMode:         cropMode,
		}

		if err := srv.store.InsertPhoto(r.Context(), &photo); err != nil {
			slog.Error("insert photo record failed", "err", err)
			writeError(w, http.StatusInternalServerError, "Failed to save photo record.")
			return
		}

		// Set computed URL fields.
		photo.ThumbURL = "/api/photos/" + photoUUID + "/thumb"
		photo.FullURL = "/api/photos/" + photoUUID + "/full"

		writeJSON(w, http.StatusCreated, photo)
	}
}

// matchMaxBytesError checks if err (or a wrapped error) is *http.MaxBytesError.
func matchMaxBytesError(err error, target **http.MaxBytesError) bool {
	// http.MaxBytesError is available in Go 1.19+.
	// ParseMultipartForm wraps the error, so we check the error message as fallback.
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "http: request body too large") ||
		strings.Contains(errMsg, "exceeds") ||
		strings.Contains(errMsg, "too large")
}

// handleServePhoto streams a photo (full or thumbnail) from storage.
func (srv *Server) handleServePhoto(thumbnail bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		photoUUID := chi.URLParam(r, "uuid")

		photo, err := srv.store.GetPhotoByUUID(r.Context(), photoUUID)
		if err != nil || photo == nil {
			http.NotFound(w, r)
			return
		}

		var pf *storage.PhotoFile
		if thumbnail {
			pf, err = srv.photos.RetrieveThumbnail(r.Context(), photo.UUID, photo.Ext)
		} else {
			pf, err = srv.photos.Retrieve(r.Context(), photo.UUID, photo.Ext)
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer pf.Reader.Close()

		w.Header().Set("Content-Type", pf.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(pf.Size, 10))
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if _, copyErr := io.Copy(w, pf.Reader); copyErr != nil {
			slog.Error("serve photo copy failed", "err", copyErr)
		}
	}
}

// handleRegenerateThumbnails re-generates thumbnails for all photos at the current thumbnail size.
func (srv *Server) handleRegenerateThumbnails() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		photos, err := srv.store.ListAllPhotos(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to list photos.")
			return
		}

		thumbSize, err := srv.store.GetThumbnailSize(r.Context())
		if err != nil || thumbSize <= 0 {
			thumbSize = 200
		}

		var regenerated, errCount int
		for _, p := range photos {
			pf, retrieveErr := srv.photos.Retrieve(r.Context(), p.UUID, p.Ext)
			if retrieveErr != nil {
				errCount++
				continue
			}

			result, processErr := imaging.ProcessUpload(pf.Reader, p.Ext, thumbSize, thumbSize, p.CropMode)
			pf.Reader.Close()
			if processErr != nil {
				errCount++
				continue
			}

			if storeErr := srv.photos.StoreThumbnail(r.Context(), p.UUID, p.Ext, result.ThumbData); storeErr != nil {
				errCount++
				continue
			}
			regenerated++
		}

		writeJSON(w, http.StatusOK, map[string]int{
			"regenerated": regenerated,
			"errors":      errCount,
		})
	}
}

// handleSetPhotosEnabled toggles the photos feature on or off.
func (srv *Server) handleSetPhotosEnabled() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON body.")
			return
		}

		if err := srv.store.SetPhotosEnabled(r.Context(), req.Enabled); err != nil {
			slog.Error("set photos_enabled failed", "err", err)
			writeError(w, http.StatusInternalServerError, "Failed to update setting.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"photos_enabled": req.Enabled})
	}
}

// handleSetThumbnailSize updates the thumbnail size setting.
// Valid range: 100-400 pixels.
func (srv *Server) handleSetThumbnailSize() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Size int `json:"size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON body.")
			return
		}

		if req.Size < 100 || req.Size > 400 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("Thumbnail size must be between 100 and 400 pixels, got %d.", req.Size))
			return
		}

		if err := srv.store.SetThumbnailSize(r.Context(), req.Size); err != nil {
			slog.Error("set thumbnail_size failed", "err", err)
			writeError(w, http.StatusInternalServerError, "Failed to update setting.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]int{"thumbnail_size": req.Size})
	}
}

// handleUnlinkPhoto detaches a photo from its item by setting item_id to NULL.
// The photo file remains in storage; only the DB association is removed.
func (srv *Server) handleUnlinkPhoto() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		photoUUID := chi.URLParam(r, "uuid")

		photo, err := srv.store.GetPhotoByUUID(r.Context(), photoUUID)
		if err != nil || photo == nil {
			http.NotFound(w, r)
			return
		}

		if err := srv.store.UnlinkPhoto(r.Context(), photoUUID); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to unlink photo.")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ImagesPageData is the view model for the images page template.
type ImagesPageData struct {
	ShelfName     string
	PhotosEnabled bool
	DisplayName   string
	AuthEnabled   bool
	Photos        []model.Photo
}

// handleImagesPage renders the images page template.
func (srv *Server) handleImagesPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFS(mustSubFS(ContentFS, "templates"),
			"layout.html", "images.html")
		if err != nil {
			slog.Error("template parse error", "err", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}

		gridData, _ := srv.store.GetGridData()
		shelfName := "Screws Box"
		authEnabled := false
		if gridData != nil {
			shelfName = gridData.ShelfName
			authEnabled = gridData.AuthEnabled
		}

		photos, err := srv.store.ListAllPhotos(r.Context())
		if err != nil {
			slog.Error("list photos failed", "err", err)
			photos = nil
		}
		// Set computed URL fields on each photo.
		for i := range photos {
			photos[i].ThumbURL = "/api/photos/" + photos[i].UUID + "/thumb"
			photos[i].FullURL = "/api/photos/" + photos[i].UUID + "/full"
		}

		data := ImagesPageData{
			ShelfName:     shelfName,
			PhotosEnabled: true, // Handler is behind requirePhotosEnabled, so always true here.
			DisplayName:   srv.getDisplayName(r),
			AuthEnabled:   authEnabled,
			Photos:        photos,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			slog.Error("template execute error", "err", err)
		}
	}
}
