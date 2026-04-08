package collector

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/jonasohland/mxl-exporter/pkg/mxl"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
)

type FlowEntry struct {
	log *slog.Logger

	domain    string    // domain filesystem path
	id        string    // flow-id
	removedAt time.Time //non-zero if the flow is not present on the host

	flow *mxl.Flow

	cachedConfig  *mxl.FlowConfigInfo
	cachedRuntime *mxl.FlowRuntimeInfo
	cachedFlowDef *mxl.FlowDefinition

	latencyGrains int64 // origin latency measured in grains last time the flow info was updated
	active        bool  // indicates that the head index had increased last time the flow info was updated
}

func NewFlowEntry(domain, id string) *FlowEntry {
	entry := &FlowEntry{
		log: slog.With("domain-path", domain, "flow-id", id),

		domain:    domain,
		id:        id,
		removedAt: time.Time{},

		flow: nil,

		cachedConfig:  nil,
		cachedRuntime: nil,
		cachedFlowDef: nil,
	}
	if err := entry.Reopen(); err != nil {
		entry.log.Warn("failed to open flow", "error", err)
	}

	return entry
}

func (f *FlowEntry) MarkRemoved() {
	f.removedAt = time.Now()
	f.Close()
}

func (f *FlowEntry) HasTimedOut(timeout time.Duration) bool {
	return !f.removedAt.IsZero() && time.Since(f.removedAt) > timeout
}

func (f *FlowEntry) Reopen() error {
	f.removedAt = time.Time{}
	if f.flow != nil {
		_ = f.flow.Close()
	}

	flow, err := mxl.Open(f.domain, f.id)
	if err != nil {
		return err
	}

	f.log.Info("flow opened")
	f.flow = flow
	f.attemptCacheRefresh()
	return nil
}

func (f *FlowEntry) attemptCacheRefresh() {
	if f.flow == nil {
		f.active = false
		return
	}

	def, err := f.flow.GetDefinition()
	if err != nil {
		f.log.Warn("failed to get flow definition")
	} else {
		f.cachedFlowDef = def
	}

	refreshTime, err := mxl.Now()
	if err != nil {
		f.log.Warn("failed to get current time", "error", err)
		return
	}

	conf, rt, err := f.flow.GetInfo()
	if err != nil {
		f.log.Warn("failed to get flow info")
	} else {
		if f.cachedRuntime != nil {
			f.active = f.cachedRuntime.HeadIndex < rt.HeadIndex
		}
		f.cachedConfig = conf
		f.cachedRuntime = rt
		f.latencyGrains = int64(mxl.TimestampToIndex(refreshTime, conf.Rate)) - int64(rt.HeadIndex)
	}
}

func (f *FlowEntry) ReopenIfInvalid() {
	if f.removedAt.IsZero() && (f.flow == nil || !f.flow.IsValid()) {
		if err := f.Reopen(); err != nil {
			f.log.Warn("failed to reopen invalid or nil flow", "error", err)
		}
	}
}

func (f *FlowEntry) Close() {
	if f.flow != nil {
		if err := f.flow.Close(); err != nil {
			f.log.Warn("failed to close flow", "error", err)
		}
		f.flow = nil
		f.log.Info("flow closed")
	}
}

func (f *FlowEntry) Collect(ch chan<- prometheus.Metric) {
	f.attemptCacheRefresh()

	if f.cachedFlowDef != nil && f.cachedConfig != nil {
		gh, err := f.cachedFlowDef.GetGroupHint()
		if err != nil {
			gh = &mxl.GroupHint{
				Name: "invalid",
				Type: "invalid",
			}
		}

		ch <- prometheus.MustNewConstMetric(Metric_FlowMetadata, prometheus.CounterValue, 1,
			f.id,
			f.domain,
			f.cachedFlowDef.Label,
			f.cachedFlowDef.Description,
			string(f.cachedConfig.Format),
			gh.Name,
			gh.Type,
			f.cachedFlowDef.Format,
			string(f.cachedConfig.PayloadLocation),
			f.cachedFlowDef.MediaType,
			f.cachedFlowDef.Colorspace,
			f.cachedFlowDef.Version,
		)
	}

	ch <- prometheus.MustNewConstMetric(Metric_FlowActive, prometheus.GaugeValue, float64(lo.Ternary(f.active, 1, 0)), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowPresent, prometheus.GaugeValue, float64(lo.Ternary(f.removedAt.IsZero(), 1, 0)), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowGrainHeadIndex, prometheus.CounterValue, float64(f.cachedRuntime.HeadIndex), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowLastReadTime, prometheus.CounterValue, float64(f.cachedRuntime.LastReadTime), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowLastWriteTime, prometheus.CounterValue, float64(f.cachedRuntime.LastWriteTime), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowLatencyGrains, prometheus.GaugeValue, float64(f.latencyGrains), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowRateDen, prometheus.CounterValue, float64(f.cachedConfig.Rate.Denominator), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowRateNum, prometheus.CounterValue, float64(f.cachedConfig.Rate.Numerator), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowMaxSyncBatchSizeHint, prometheus.CounterValue, float64(f.cachedConfig.MaxSyncBatchSizeHint), f.id, f.domain)
	ch <- prometheus.MustNewConstMetric(Metric_FlowMaxCommitBatchSizeHint, prometheus.CounterValue, float64(f.cachedConfig.MaxCommitBatchSizeHint), f.id, f.domain)

	if f.cachedConfig.Discrete != nil {
		for index, size := range f.cachedConfig.Discrete.SliceSizes {
			ch <- prometheus.MustNewConstMetric(Metric_FlowSliceSizes, prometheus.CounterValue, float64(size), f.id, f.domain, strconv.Itoa(index))
		}
		ch <- prometheus.MustNewConstMetric(Metric_FlowBufferGrains, prometheus.CounterValue, float64(f.cachedConfig.Discrete.GrainCount), f.id, f.domain)
	}

	if f.cachedConfig.Continuous != nil {
		ch <- prometheus.MustNewConstMetric(Metric_FlowChannels, prometheus.CounterValue, float64(f.cachedConfig.Continuous.Channels), f.id, f.domain)
		ch <- prometheus.MustNewConstMetric(Metric_FlowBufferSize, prometheus.CounterValue, float64(f.cachedConfig.Continuous.BufferLength), f.id, f.domain)
	}
}
