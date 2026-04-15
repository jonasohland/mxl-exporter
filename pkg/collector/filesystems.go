package collector

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sys/unix"
)

var fsMetricLabels = []string{
	"fs_path",
	"domain",
}

var (
	Metric_DomainFsSpaceTotal     = prometheus.NewDesc("mxl_domain_fs_space_total_bytes", "Bytes available on the domain filesystem", fsMetricLabels, nil)
	Metric_DomainFsSpaceAvailable = prometheus.NewDesc("mxl_domain_fs_space_available_bytes", "Bytes available on the domain filesystem", fsMetricLabels, nil)
	Metric_DomainFsSpaceUsed      = prometheus.NewDesc("mxl_domain_fs_space_used_bytes", "Bytes available on the domain filesystem", fsMetricLabels, nil)
)

type fsentry struct {
	removedAt time.Time
	domains   []string
}

type FilesystemCollector struct {
	mu       sync.Mutex
	fs       map[string]*fsentry
	lifetime time.Duration
}

func getFilesystemSpace(path string) (uint64, uint64, uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, 0, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	return total, avail, total - avail, nil
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

func (fc *FilesystemCollector) UpdateFilesystem(path string, domains []string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	entry, ok := fc.fs[path]
	if !ok {
		return
	}

	entry.domains = slices.Clone(domains)
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

func (fc *FilesystemCollector) Describe(ch chan<- *prometheus.Desc) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

}

func (fc *FilesystemCollector) Collect(ch chan<- prometheus.Metric) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	for path, entry := range fc.fs {
		total, avail, used, err := getFilesystemSpace(path)
		if err != nil {
			slog.Error("failed to get filesystem stats", "filesystem-path", path, "error", err)
			continue
		}
		for _, domain := range entry.domains {
			ch <- prometheus.MustNewConstMetric(Metric_DomainFsSpaceTotal, prometheus.GaugeValue, float64(total), path, domain)
			ch <- prometheus.MustNewConstMetric(Metric_DomainFsSpaceUsed, prometheus.GaugeValue, float64(used), path, domain)
			ch <- prometheus.MustNewConstMetric(Metric_DomainFsSpaceAvailable, prometheus.GaugeValue, float64(avail), path, domain)
		}
	}
}
