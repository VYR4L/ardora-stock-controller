package components

import (
	"os"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/frontend/icons"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

type imageEntry struct {
	bitmap *ui.Bitmap
	valid  bool
}

// ThumbnailCache avoids decoding the same local product/game image every frame.
type ThumbnailCache struct {
	mu       sync.Mutex
	entries  map[string]imageEntry
	order    []string
	capacity int
}

func NewThumbnailCache(capacity int) *ThumbnailCache {
	if capacity < 1 {
		capacity = 64
	}
	return &ThumbnailCache{entries: make(map[string]imageEntry), capacity: capacity}
}

func (c *ThumbnailCache) Load(path string) (*ui.Bitmap, bool) {
	if path == "" {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[path]; ok {
		return entry.bitmap, entry.valid
	}

	data, err := os.ReadFile(path)
	if err != nil {
		c.store(path, imageEntry{})
		return nil, false
	}
	bitmap, err := ui.DecodeBitmap(data)
	if err != nil {
		c.store(path, imageEntry{})
		return nil, false
	}
	entry := imageEntry{bitmap: bitmap, valid: true}
	c.store(path, entry)
	return bitmap, true
}

func (c *ThumbnailCache) store(path string, entry imageEntry) {
	if len(c.entries) >= c.capacity && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
	c.entries[path] = entry
	c.order = append(c.order, path)
}

var thumbnails = NewThumbnailCache(96)

func Thumbnail(c *ui.Context, path, label string, size float32) ui.Element {
	if size <= 0 {
		size = 52
	}
	tokens := style.FromContext(c)
	container := ui.Box(c).Size(size, size).Radius(10).Clip().Background(tokens.SurfaceSubtle).Border(1, tokens.Border).Label(label)
	return container.Children(func() {
		if bitmap, ok := thumbnails.Load(path); ok {
			ui.Image(c, bitmap).Size(size, size).Fit(ui.Cover)
			return
		}
		ui.Icon(c, icons.Image).Size(size*0.42, size*0.42).Margin(size * 0.29).TextColor(tokens.Muted)
	})
}
