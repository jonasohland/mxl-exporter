package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type fsentry struct {
	removedAt time.Time
}

type FilesystemCollector struct {
	mu       sync.Mutex
	fs       map[string]*fsentry
	lifetime time.Duration
}

func NewFilesystemCollector(ctx context.Context, wg *sync.WaitGroup, lifetime time.Duration) *FilesystemCollector {
	fc := &FilesystemCollector{mu: sync.Mutex{}, fs: map[string]*fsentry{}, lifetime: lifetime}
	wg.Go(func() { fc.run(ctx) })
	return fc
}

func (fc *FilesystemCollector) AddFilesystem(path string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.fs[path] = &fsentry{removedAt: time.Time{}}
}

func (fc *FilesystemCollector) RemoveFilesystem(path string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	entry, ok := fc.fs[path]
	if !ok {
		return
	}

	entry.removedAt = time.Now()
}

func (fc *FilesystemCollector) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(7 * time.Second):
			fc.gc()
		}
	}
}

func (fc *FilesystemCollector) gc() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	for path, fs := range fc.fs {
		if !fs.removedAt.IsZero() &&
			time.Since(fs.removedAt) > fc.lifetime {
			slog.Debug("filesystem entry timed out", "filesystem-path", path)
			delete(fc.fs, path)
		}
	}
}
