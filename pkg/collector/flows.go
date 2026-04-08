package collector

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var flowMetricLabels = []string{
	"flow_id",
	"domain",
}

var flowMetadataLabels = []string{
	"flow_id",
	"domain",
	"flow_label",
	"flow_description",
	"flow_group_name",
	"flow_group_type",
	"flow_data_type",
	"flow_format",
	"flow_payload_location",
	"flow_media_type",
	"flow_colorspace",
	"flow_version",
}

var (
	Metric_FlowMetadata = prometheus.NewDesc("mxl_flow_metadata", "Dummy metric with all the flow metadata as labels", flowMetadataLabels, nil)
	Metric_FlowPresent  = prometheus.NewDesc("mxl_flow_present", "Indicates that the flow is present on in the domain", flowMetricLabels, nil)
	Metric_FlowActive   = prometheus.NewDesc("mxl_flow_active", "Indicates that the flow is active", flowMetricLabels, nil)

	Metric_FlowGrainHeadIndex = prometheus.NewDesc("mxl_flow_head_index_grains", "Current head index of the flow", flowMetricLabels, nil)
	Metric_FlowLastReadTime   = prometheus.NewDesc("mxl_flow_last_read_time_ns", "Last read time in nanoseconds since epoch", flowMetricLabels, nil)
	Metric_FlowLastWriteTime  = prometheus.NewDesc("mxl_flow_last_write_time_ns", "Last write time in nanoseconds since epoch", flowMetricLabels, nil)
	Metric_FlowLatencyGrains  = prometheus.NewDesc("mxl_flow_latency_grains", "Current flow latency in grains", flowMetricLabels, nil)

	Metric_FlowRateDen                = prometheus.NewDesc("mxl_flow_rate_den", "Flow rate denominator", flowMetricLabels, nil)
	Metric_FlowRateNum                = prometheus.NewDesc("mxl_flow_rate_num", "Flow rate numerator", flowMetricLabels, nil)
	Metric_FlowMaxCommitBatchSizeHint = prometheus.NewDesc("mxl_flow_max_commit_batch_size_hint", "Flow maximum commit batch size hint", flowMetricLabels, nil)
	Metric_FlowMaxSyncBatchSizeHint   = prometheus.NewDesc("mxl_flow_max_sync_batch_size_hint", "Flow maximum sync batch size hint", flowMetricLabels, nil)

	Metric_FlowSliceSizes   = prometheus.NewDesc("mxl_flow_payload_slice_size_bytes", "Size of a payload buffer slice in bytes", append(flowMetricLabels, "payload_buffer_index"), nil)
	Metric_FlowBufferGrains = prometheus.NewDesc("mxl_flow_ring_buffer_size_grains", "Size of the flow ring buffer in grains", flowMetricLabels, nil)
	Metric_FlowFramWidth    = prometheus.NewDesc("mxl_flow_frame_width", "Width of a single frame of the flow in pixels", flowMetricLabels, nil)

	Metric_FlowChannels   = prometheus.NewDesc("mxl_flow_channels_total", "Number of channels in the flow", flowMetricLabels, nil)
	Metric_FlowBufferSize = prometheus.NewDesc("mxl_flow_ring_buffer_size_samples", "Length of the flow buffer in samples", flowMetricLabels, nil)
)

type FlowCollector struct {
	mu       sync.Mutex
	flows    map[string]*FlowEntry
	lifetime time.Duration
}

func NewFlowCollector(ctx context.Context, wg *sync.WaitGroup, lifetime time.Duration) *FlowCollector {
	coll := &FlowCollector{mu: sync.Mutex{}, flows: map[string]*FlowEntry{}, lifetime: lifetime}
	wg.Go(func() { coll.runGC(ctx) })
	return coll
}

func (f *FlowCollector) AddFlow(domain, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	fullpath := filepath.Join(domain, id+".mxl-flow")
	existing, ok := f.flows[fullpath]
	if ok {
		if err := existing.Reopen(); err != nil {
			slog.Warn("failed to re-open flow", "domain-path", domain, "flow-id", id)
		}
		return
	}

	f.flows[fullpath] = NewFlowEntry(domain, id)
}

func (f *FlowCollector) RemoveFlow(domain, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	fullpath := filepath.Join(domain, id+".mxl-flow")
	flow, ok := f.flows[fullpath]
	if !ok {
		return
	}

	flow.MarkRemoved()
}

func (f *FlowCollector) runGC(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(7 * time.Second):
			f.gc()
		}
	}
}

func (f *FlowCollector) gc() {
	f.mu.Lock()
	defer f.mu.Unlock()

	cleanup := make([]string, 0)

	for fullpath, entry := range f.flows {
		if entry.HasTimedOut(f.lifetime) {
			entry.log.Debug("flow entry timed out")
			cleanup = append(cleanup, fullpath)
		}
	}

	for _, fullpath := range cleanup {
		entry := f.flows[fullpath]
		entry.Close()
		delete(f.flows, fullpath)
	}

	for _, entry := range f.flows {
		entry.ReopenIfInvalid()
	}
}

func (f *FlowCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- Metric_FlowMetadata
	ch <- Metric_FlowActive
	ch <- Metric_FlowPresent
	ch <- Metric_FlowGrainHeadIndex
	ch <- Metric_FlowLastReadTime
	ch <- Metric_FlowLastWriteTime
	ch <- Metric_FlowLatencyGrains
	ch <- Metric_FlowRateDen
	ch <- Metric_FlowRateNum
	ch <- Metric_FlowMaxCommitBatchSizeHint
	ch <- Metric_FlowMaxSyncBatchSizeHint
	ch <- Metric_FlowChannels
	ch <- Metric_FlowBufferSize
}

func (f *FlowCollector) Collect(ch chan<- prometheus.Metric) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, flow := range f.flows {
		flow.Collect(ch)
	}
}
