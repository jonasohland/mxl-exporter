package collector

import (
	"time"
)

type DomainEntry struct {
	removedAt time.Time
	path      string
}

func NewDomainEntry(path string) *DomainEntry {
	return &DomainEntry{removedAt: time.Time{}, path: path}
}

func (d *DomainEntry) MarkRemoved() {
	d.removedAt = time.Now()
}

func (d *DomainEntry) UnmarkRemoved() {
	d.removedAt = time.Time{}
}

func (d *DomainEntry) IsExpired(timeout time.Duration) bool {
	return !d.removedAt.IsZero() && time.Since(d.removedAt) > timeout
}
