package imaging

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"github.com/disintegration/imaging"
)

// ProcessResult holds the re-encoded full-size image and its thumbnail.
type ProcessResult struct {
	FullData  *bytes.Buffer
	ThumbData *bytes.Buffer
}

// ProcessUpload decodes an image with EXIF auto-orientation, re-encodes the
// full-size image (stripping all EXIF metadata), and generates a thumbnail.
//
// ext must include a leading dot (e.g., ".jpg", ".png").
// cropMode is "fit" (default, preserves aspect ratio) or "crop" (center-crop to exact size).
// thumbWidth and thumbHeight are the target thumbnail dimensions in pixels.
//
// Both FullData and ThumbData in the result are ready to pass to PhotoStorage.Store/StoreThumbnail.
func ProcessUpload(r io.Reader, ext string, thumbWidth, thumbHeight int, cropMode string) (*ProcessResult, error) {
	format, err := imaging.FormatFromExtension(ext)
	if err != nil {
		return nil, fmt.Errorf("unsupported image format %q: %w", ext, err)
	}

	// Decode with auto-orientation (fixes EXIF rotation tags)
	img, err := imaging.Decode(r, imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Re-encode full-size (strips ALL EXIF metadata by design)
	fullBuf := new(bytes.Buffer)
	if err := imaging.Encode(fullBuf, img, format); err != nil {
		return nil, fmt.Errorf("encode full-size: %w", err)
	}

	// Generate thumbnail
	var thumb *image.NRGBA
	if cropMode == "crop" {
		thumb = imaging.Fill(img, thumbWidth, thumbHeight, imaging.Center, imaging.Lanczos)
	} else {
		thumb = imaging.Fit(img, thumbWidth, thumbHeight, imaging.Lanczos)
	}

	thumbBuf := new(bytes.Buffer)
	if err := imaging.Encode(thumbBuf, thumb, format); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}

	return &ProcessResult{
		FullData:  fullBuf,
		ThumbData: thumbBuf,
	}, nil
}
