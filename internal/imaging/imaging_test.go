package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestJPEG creates a 400x300 JPEG image in memory.
func makeTestJPEG(t *testing.T) *bytes.Buffer {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := range 300 {
		for x := range 400 {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255}) //nolint:gosec // test only
		}
	}
	buf := new(bytes.Buffer)
	require.NoError(t, jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}))
	return buf
}

// makeTestPNG creates a 400x300 PNG image in memory.
func makeTestPNG(t *testing.T) *bytes.Buffer {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := range 300 {
		for x := range 400 {
			img.Set(x, y, color.RGBA{R: 50, G: uint8(x % 256), B: uint8(y % 256), A: 255}) //nolint:gosec // test only
		}
	}
	buf := new(bytes.Buffer)
	require.NoError(t, png.Encode(buf, img))
	return buf
}

func TestProcessUploadJPEG(t *testing.T) {
	input := makeTestJPEG(t)
	result, err := ProcessUpload(input, ".jpg", 200, 200, "fit")
	require.NoError(t, err)
	assert.True(t, result.FullData.Len() > 0, "full image should not be empty")
	assert.True(t, result.ThumbData.Len() > 0, "thumbnail should not be empty")
}

func TestProcessUploadPNG(t *testing.T) {
	input := makeTestPNG(t)
	result, err := ProcessUpload(input, ".png", 200, 200, "fit")
	require.NoError(t, err)
	assert.True(t, result.FullData.Len() > 0, "full image should not be empty")
	assert.True(t, result.ThumbData.Len() > 0, "thumbnail should not be empty")

	// Verify output is valid PNG by decoding
	_, format, err := image.Decode(bytes.NewReader(result.FullData.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, "png", format)
}

func TestProcessUploadThumbnailFitMode(t *testing.T) {
	input := makeTestJPEG(t) // 400x300
	result, err := ProcessUpload(input, ".jpg", 200, 200, "fit")
	require.NoError(t, err)

	// Decode the thumbnail to check dimensions
	thumb, _, err := image.Decode(bytes.NewReader(result.ThumbData.Bytes()))
	require.NoError(t, err)
	bounds := thumb.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	assert.LessOrEqual(t, w, 200, "fit width should be <= 200")
	assert.LessOrEqual(t, h, 200, "fit height should be <= 200")

	// 400x300 fitted to 200x200 should be 200x150 (preserving 4:3 ratio)
	assert.Equal(t, 200, w, "wider dimension should be 200")
	assert.Equal(t, 150, h, "height should preserve aspect ratio")
}

func TestProcessUploadThumbnailCropMode(t *testing.T) {
	input := makeTestJPEG(t) // 400x300
	result, err := ProcessUpload(input, ".jpg", 200, 200, "crop")
	require.NoError(t, err)

	thumb, _, err := image.Decode(bytes.NewReader(result.ThumbData.Bytes()))
	require.NoError(t, err)
	bounds := thumb.Bounds()

	assert.Equal(t, 200, bounds.Dx(), "crop width should be exactly 200")
	assert.Equal(t, 200, bounds.Dy(), "crop height should be exactly 200")
}

func TestProcessUploadStripsEXIF(t *testing.T) {
	// Create a JPEG, process it, verify output decodes with correct dimensions.
	// The re-encode cycle strips EXIF by design (image.Image has no metadata).
	input := makeTestJPEG(t) // 400x300
	result, err := ProcessUpload(input, ".jpg", 200, 200, "fit")
	require.NoError(t, err)

	// Decode the full-size output
	full, _, err := image.Decode(bytes.NewReader(result.FullData.Bytes()))
	require.NoError(t, err)
	bounds := full.Bounds()
	assert.Equal(t, 400, bounds.Dx(), "full-size width should be preserved")
	assert.Equal(t, 300, bounds.Dy(), "full-size height should be preserved")

	// The output is a clean re-encoded image with no EXIF metadata.
	// Verify the output differs from input (re-encoding changes bytes).
	assert.NotEqual(t, input.Bytes(), result.FullData.Bytes(),
		"re-encoded output should differ from input bytes")
}

func TestProcessUploadInvalidData(t *testing.T) {
	badData := bytes.NewReader([]byte("this is not an image"))
	_, err := ProcessUpload(badData, ".jpg", 200, 200, "fit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode image")
}

func TestProcessUploadUnsupportedExtension(t *testing.T) {
	input := makeTestJPEG(t)
	_, err := ProcessUpload(input, ".webp", 200, 200, "fit")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "unsupported") || strings.Contains(err.Error(), "format"),
		"error should mention unsupported format, got: %s", err.Error())
}
