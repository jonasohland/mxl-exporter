package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
