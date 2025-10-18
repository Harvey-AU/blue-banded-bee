package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config controls observability initialisation.
type Config struct {
	Enabled        bool
	ServiceName    string
	Environment    string
	OTLPEndpoint   string
	OTLPHeaders    map[string]string
	OTLPInsecure   bool
	MetricsAddress string
}

// Providers exposes configured telemetry providers.
type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	Propagator     propagation.TextMapPropagator
	MetricsHandler http.Handler
	Shutdown       func(ctx context.Context) error
	Config         Config
}

var (
	initOnce sync.Once

	workerTracer trace.Tracer

	workerTaskDuration metric.Float64Histogram
	workerTaskTotal    metric.Int64Counter
)

// Init configures tracing and metrics exporters. When cfg.Enabled is false the function is a no-op.
func Init(ctx context.Context, cfg Config) (*Providers, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = "blue-banded-bee"
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	var spanExporter sdktrace.SpanExporter
	if cfg.OTLPEndpoint != "" {
		clientOpts := []otlptracehttp.Option{
			getOTLPEndpointOption(cfg.OTLPEndpoint),
		}
		if cfg.OTLPInsecure {
			clientOpts = append(clientOpts, otlptracehttp.WithInsecure())
		}
		if len(cfg.OTLPHeaders) > 0 {
			clientOpts = append(clientOpts, otlptracehttp.WithHeaders(cfg.OTLPHeaders))
		}

		exp, err := otlptracehttp.New(ctx, clientOpts...)
		if err != nil {
			// Log error but don't fail app startup - observability is optional
			fmt.Printf("WARN: Failed to create OTLP trace exporter (traces disabled): %v\n", err)
			fmt.Printf("WARN: Endpoint: %s\n", cfg.OTLPEndpoint)
			// Continue without tracing - app should still function
		} else {
			spanExporter = exp
			fmt.Printf("INFO: OTLP trace exporter initialised successfully for endpoint: %s\n", cfg.OTLPEndpoint)
		}
	}

	traceOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}
	if spanExporter != nil {
		traceOpts = append(traceOpts, sdktrace.WithBatcher(spanExporter))
	}

	tracerProvider := sdktrace.NewTracerProvider(traceOpts...)
	otel.SetTracerProvider(tracerProvider)

	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)
	promExporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
	)
	if err != nil {
		_ = tracerProvider.Shutdown(ctx) // best-effort cleanup
		return nil, fmt.Errorf("create Prometheus exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)
	otel.SetMeterProvider(meterProvider)

	initOnce.Do(func() {
		workerTracer = tracerProvider.Tracer("blue-banded-bee/worker")
		_ = initWorkerInstruments(meterProvider)
	})

	shutdown := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		var allErr error
		if err := meterProvider.Shutdown(ctx); err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("metric provider shutdown: %w", err))
		}
		if err := tracerProvider.Shutdown(ctx); err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("trace provider shutdown: %w", err))
		}
		return allErr
	}

	return &Providers{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		Propagator:     prop,
		MetricsHandler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		Shutdown:       shutdown,
		Config:         cfg,
	}, nil
}

func getOTLPEndpointOption(endpoint string) otlptracehttp.Option {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return otlptracehttp.WithEndpointURL(endpoint)
	}
	return otlptracehttp.WithEndpoint(endpoint)
}

// WrapHandler applies OpenTelemetry instrumentation to an http.Handler when the providers are active.
func WrapHandler(handler http.Handler, prov *Providers) http.Handler {
	if prov == nil || prov.TracerProvider == nil {
		return handler
	}

	options := []otelhttp.Option{
		otelhttp.WithTracerProvider(prov.TracerProvider),
		otelhttp.WithPropagators(prov.Propagator),
		otelhttp.WithMeterProvider(prov.MeterProvider),
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		}),
		// Skip tracing for health checks to reduce noise
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/health"
		}),
	}

	return otelhttp.NewHandler(handler, "http.server", options...)
}

func initWorkerInstruments(meterProvider *sdkmetric.MeterProvider) error {
	if meterProvider == nil {
		return nil
	}

	meter := meterProvider.Meter("blue-banded-bee/worker")

	var err error
	workerTaskDuration, err = meter.Float64Histogram(
		"bee.worker.task.duration_ms",
		metric.WithUnit("ms"),
		metric.WithDescription("Time taken to process a cache warming task"),
	)
	if err != nil {
		return err
	}

	workerTaskTotal, err = meter.Int64Counter(
		"bee.worker.task.total",
		metric.WithDescription("Counts task outcomes processed by the worker pool"),
	)
	return err
}

// WorkerTaskSpanInfo describes the attributes used when starting a worker task span.
type WorkerTaskSpanInfo struct {
	JobID     string
	TaskID    string
	Domain    string
	Path      string
	FindLinks bool
}

// WorkerTaskMetrics describes a processed task for metric recording.
type WorkerTaskMetrics struct {
	JobID    string
	Status   string
	Duration time.Duration
}

// StartWorkerTaskSpan starts a span for an individual worker task.
func StartWorkerTaskSpan(ctx context.Context, info WorkerTaskSpanInfo) (context.Context, trace.Span) {
	t := workerTracer
	if t == nil {
		t = otel.Tracer("blue-banded-bee/worker")
	}

	attrs := []attribute.KeyValue{
		attribute.String("job.id", info.JobID),
		attribute.String("task.id", info.TaskID),
		attribute.String("task.domain", info.Domain),
		attribute.String("task.path", info.Path),
		attribute.Bool("task.find_links", info.FindLinks),
	}

	return t.Start(ctx, "worker.process_task", trace.WithAttributes(attrs...))
}

// RecordWorkerTask emits worker task metrics when instrumentation is initialised.
func RecordWorkerTask(ctx context.Context, metrics WorkerTaskMetrics) {
	if workerTaskDuration != nil {
		workerTaskDuration.Record(ctx, float64(metrics.Duration.Milliseconds()),
			metric.WithAttributes(attribute.String("job.id", metrics.JobID), attribute.String("task.status", metrics.Status)))
	}

	if workerTaskTotal != nil {
		workerTaskTotal.Add(ctx, 1,
			metric.WithAttributes(attribute.String("job.id", metrics.JobID), attribute.String("task.status", metrics.Status)))
	}
}
