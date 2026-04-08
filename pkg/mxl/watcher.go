package mxl

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/samber/lo"
)

type FlowReceiver interface {
	AddFlow(domain, id string)
	RemoveFlow(domain, id string)
}

type watcherEntry struct {
	flows []string
}

type Watcher struct {
	mu     sync.Mutex
	notify *fsnotify.Watcher
	cache  map[string]*watcherEntry
	recv   []FlowReceiver
}

func NewWatcher(ctx context.Context, wg *sync.WaitGroup, recv []FlowReceiver) (*Watcher, error) {
	notify, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		mu:     sync.Mutex{},
		notify: notify,
		cache:  map[string]*watcherEntry{},
		recv:   recv,
	}

	wg.Go(func() { w.run(ctx) })

	return w, nil
}

func (w *Watcher) AddDomain(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.notify.Add(path); err != nil {
		slog.Error("failed to add inotify watch", "path", path, "error", err)
	}

	_, ok := w.cache[path]
	if ok {
		slog.Warn("received domain discovery notification for a known domain")
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		slog.Error("failed to read domain directory entries", "error", err)
		return
	}

	flows := make([]string, 0)
	for _, entry := range entries {
		id, ok := getFlowIDFromPath(entry.Name())
		if ok {
			flows = append(flows, id)
		}
	}

	w.cache[path] = &watcherEntry{flows: flows}

	for _, id := range flows {
		w.onFlowAdded(path, id)
	}
}

func (w *Watcher) RemoveDomain(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.notify.Remove(path); err != nil {
		slog.Error("failed to remove inotify watch", "path", path, "error", err)
	}

	entry, ok := w.cache[path]
	if !ok {
		return
	}

	for _, id := range entry.flows {
		w.onFlowRemoved(path, id)
	}

	delete(w.cache, path)
}

func (w *Watcher) run(ctx context.Context) {
	for {
		select {
		case ev, ok := <-w.notify.Events:
			if !ok {
				return
			}
			if !ev.Op.Has(fsnotify.Create) && !ev.Op.Has(fsnotify.Remove) {
				continue
			}

			if isFlowDir(ev.Name) {
				if ev.Op.Has(fsnotify.Create) {
					w.flowAddedEvent(ev.Name)
				} else if ev.Op.Has(fsnotify.Remove) {
					w.flowRemovedEvent(ev.Name)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *Watcher) flowAddedEvent(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	domain := filepath.Dir(path)
	entry, ok := w.cache[domain]
	if !ok {
		return
	}

	id, ok := getFlowIDFromPath(path)
	if !ok {
		return
	}

	if !lo.Contains(entry.flows, id) {
		entry.flows = append(entry.flows, id)
		w.onFlowAdded(domain, id)
	}
}

func (w *Watcher) flowRemovedEvent(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	domain := filepath.Dir(path)
	entry, ok := w.cache[domain]
	if !ok {
		return
	}

	id, ok := getFlowIDFromPath(path)
	if !ok {
		return
	}

	if lo.Contains(entry.flows, id) {
		entry.flows = lo.Without(entry.flows, id)
		w.onFlowRemoved(domain, id)
	}
}

func (w *Watcher) onFlowAdded(domain, id string) {
	for _, recv := range w.recv {
		recv.AddFlow(domain, id)
	}
}

func (w *Watcher) onFlowRemoved(domain, id string) {
	for _, recv := range w.recv {
		recv.RemoveFlow(domain, id)
	}
}
