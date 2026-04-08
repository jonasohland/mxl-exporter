package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/jonasohland/mxl-exporter/pkg/collector"
	"github.com/jonasohland/mxl-exporter/pkg/mxl"
	"github.com/jonasohland/mxl-exporter/pkg/server"
	"github.com/prometheus/client_golang/prometheus"
)

type Options struct {
	Listen string   `short:"l" default:"127.0.0.1:2284"`
	Search []string `short:"s"`
	Domain []string `short:"d"`

	DefaultLifetime time.Duration  `default:"24h" help:"Default duration to keep entries of any kind around after they are no longer detected on the system."`
	FSLifetime      *time.Duration `help:"Duration to keep filesystem entries around after they are no longer used"`
	DomainLifetime  *time.Duration
	FlowLifetime    *time.Duration
}

type LoggingDomainRecevier struct{}
type LoggingFlowReceiver struct{}
type LoggingFilesytemReceiver struct{}

func (l *LoggingDomainRecevier) AddDomain(path string) {
	slog.Info("domain discovered", "domain-path", path)
}

func (l *LoggingDomainRecevier) RemoveDomain(path string) {
	slog.Info("domain removed", "domain-path", path)
}

func (l *LoggingFlowReceiver) AddFlow(domain, id string) {
	slog.Info("flow discovered", "domain-path", domain, "flow-id", id)
}

func (l *LoggingFlowReceiver) RemoveFlow(domain, id string) {
	slog.Info("flow removed", "domain-path", domain, "flow-id", id)
}

func (l *LoggingFilesytemReceiver) AddFilesystem(path string) {
	slog.Info("filesystem discovered", "filesystem-path", path)
}

func (l *LoggingFilesytemReceiver) RemoveFilesystem(path string) {
	slog.Info("filesystem removed", "filesystem-path", path)
}

func orDefaultDuration(dd time.Duration, v *time.Duration) time.Duration {
	if v == nil {
		return dd
	}

	return *v
}

func launch(ctx context.Context, wg *sync.WaitGroup, opts *Options) error {
	reg := prometheus.NewRegistry()
	srv := server.NewServer(wg, ctx)
	metrics := server.NewMetricsService(reg)
	flowCollector := collector.NewFlowCollector(ctx, wg, orDefaultDuration(opts.DefaultLifetime, opts.FlowLifetime))
	domainCollector := collector.NewDomainCollector(ctx, wg, orDefaultDuration(opts.DefaultLifetime, opts.DomainLifetime))
	fsCollector := collector.NewFilesystemCollector(ctx, wg, orDefaultDuration(opts.DefaultLifetime, opts.FSLifetime))

	srv.Mux(metrics)

	if err := reg.Register(flowCollector); err != nil {
		return err
	}

	roots := mxl.NewFilesystemDiscoverer([]mxl.FilesytemReceiver{
		&LoggingFilesytemReceiver{},
		fsCollector,
	})

	watcher, err := mxl.NewWatcher(ctx, wg,
		[]mxl.FlowReceiver{
			&LoggingFlowReceiver{},
			flowCollector,
		})
	if err != nil {
		return err
	}

	mxl.NewDiscoverer(
		ctx, wg,
		[]mxl.DomainReceiver{
			&LoggingDomainRecevier{},
			watcher,
			domainCollector,
			roots,
		},
		opts.Search, opts.Domain)

	return srv.StartListening(opts.Listen, nil)
}

func main() {
	var opts Options
	kong.Parse(&opts)

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	wg := &sync.WaitGroup{}
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := launch(ctx, wg, &opts); err != nil {
		cancel()
		wg.Wait()
		os.Exit(1)
	}

	<-ctx.Done()
	wg.Wait()
}
