package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ImageService handles copying and storing uploaded images.
type ImageService struct {
	imagesDir string
}

func NewImageService(imagesDir string) *ImageService {
	return &ImageService{imagesDir: imagesDir}
}

// StoreCopy copies an existing file into the local images directory with a UUID name.
// It returns the new base filename (e.g. "123e4567-e89b-12d3-a456-426614174000.jpg").
func (s *ImageService) StoreCopy(sourcePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(sourcePath))
	if ext == "" {
		ext = ".jpg" // default fallback
	}

	newID := uuid.New().String()
	newName := newID + ext
	destPath := filepath.Join(s.imagesDir, newName)

	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open source image: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create dest image: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", fmt.Errorf("copy image: %w", err)
	}

	return newName, nil
}

// GetPath returns the absolute path for an image filename.
func (s *ImageService) GetPath(filename string) string {
	if filename == "" {
		return ""
	}
	return filepath.Join(s.imagesDir, filename)
}
