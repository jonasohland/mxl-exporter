package collector

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jonasohland/mxl-exporter/pkg/mxl"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
)

var domainMetricLabels = []string{
	"domain",
}

var (
	Metric_DomainSizeBytes = prometheus.NewDesc("mxl_domain_size_bytes", "Size of all data in the domain in bytes", domainMetricLabels, nil)
	Metric_DomainNumFlows  = prometheus.NewDesc("mxl_domain_num_flows", "Size of all data in the domain in bytes", domainMetricLabels, nil)
)

type DomainCollector struct {
	mu       sync.Mutex
	domains  map[string]*DomainEntry
	lifetime time.Duration
}

func getDomainStats(domainPath string) (int, uint64, error) {
	entries, err := os.ReadDir(domainPath)
	if err != nil {
		return 0, 0, err
	}

	totalSize := int64(0)
	flowDirs := lo.Filter(entries, func(item fs.DirEntry, _ int) bool { return mxl.IsFlowDir(item.Name()) })
	for _, flowDir := range flowDirs {
		flowPath := filepath.Join(domainPath, flowDir.Name())
		flowFS := os.DirFS(flowPath)
		err := fs.WalkDir(flowFS, ".",
			func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}

				info, err := d.Info()
				if err != nil {
					return err
				}

				totalSize += info.Size()
				return nil
			})
		if err != nil {
			return 0, 0, err
		}
	}

	return len(flowDirs), uint64(totalSize), nil
}

func NewDomainCollector(ctx context.Context, wg *sync.WaitGroup, lifetime time.Duration) *DomainCollector {
	dc := &DomainCollector{mu: sync.Mutex{}, domains: map[string]*DomainEntry{}, lifetime: lifetime}
	wg.Go(func() { dc.run(ctx) })
	return dc
}

func (d *DomainCollector) AddDomain(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	existing, ok := d.domains[path]
	if ok {
		existing.UnmarkRemoved()
		return
	}

	d.domains[path] = NewDomainEntry(path)
}

func (d *DomainCollector) RemoveDomain(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.domains[path]
	if !ok {
		return
	}

	entry.MarkRemoved()
}

func (d *DomainCollector) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(7 * time.Second):
			d.gc()
		}
	}
}

func (d *DomainCollector) gc() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for path, entry := range d.domains {
		if entry.IsExpired(d.lifetime) {
			slog.Debug("domain entry timed out", "domain-path", path)
			delete(d.domains, path)
		}
	}
}

func (d *DomainCollector) Describe(ch chan<- *prometheus.Desc) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ch <- Metric_DomainSizeBytes
	ch <- Metric_DomainNumFlows
}

func (d *DomainCollector) Collect(ch chan<- prometheus.Metric) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, domain := range d.domains {
		numFlows, spaceUsed, err := getDomainStats(domain.path)
		if err != nil {
			slog.Error("failed to get domain stats", "domain", domain.path, "error", err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(Metric_DomainNumFlows, prometheus.GaugeValue, float64(numFlows), domain.path)
		ch <- prometheus.MustNewConstMetric(Metric_DomainSizeBytes, prometheus.GaugeValue, float64(spaceUsed), domain.path)
	}
}
