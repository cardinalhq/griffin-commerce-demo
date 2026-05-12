// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common/logging"
	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/host"
	iruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	commonAttributes attribute.Set
	meter            = otel.Meter("github.com/cardinalhq/griffin-commerce-demo")
)

// SetupTelemetry sets up OpenTelemetry SDK with support for OTLP export
// Returns a context, shutdown function, and error
func SetupTelemetry(serviceName string, addlAttrs *attribute.Set) (context.Context, func() error, error) {
	// Set up signal handling for graceful shutdown
	ctx, cancel := handleSignals(context.Background())

	// Default shutdown function
	shutdownFunc := func() error {
		return nil
	}

	// Process additional attributes
	attrs := []attribute.KeyValue{}
	if addlAttrs != nil {
		iter := addlAttrs.Iter()
		for iter.Next() {
			attrs = append(attrs, iter.Attribute())
		}
	}
	commonAttributes = attribute.NewSet(attrs...)

	// Configure slog level based on DEBUG environment variables
	var opts *slog.HandlerOptions
	if os.Getenv("GRIFFIN_DEBUG") != "" {
		opts = &slog.HandlerOptions{Level: slog.LevelDebug}
	}

	var logger *slog.Logger
	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if os.Getenv("OTEL_SERVICE_NAME") != "" && otelEndpoint != "" {
		// Set up multi-handler logging (console + OTLP).
		// Stdout is wrapped with TraceHandler so trace_id/span_id appear in
		// `kubectl logs` output. otelslog already attaches trace context to
		// OTLP log records when callers use slog.*Context variants.
		logger = slog.New(slogmulti.Fanout(
			logging.NewTraceHandler(slog.NewTextHandler(os.Stdout, opts)),
			otelslog.NewHandler(serviceName),
		)).With(
			slog.String("service", serviceName),
		)
		slog.SetDefault(logger)

		// Setup OpenTelemetry SDK
		otelShutdown, err := setupOTelSDK(ctx)
		if err != nil {
			cancel()
			return ctx, nil, fmt.Errorf("failed to setup OpenTelemetry SDK: %w", err)
		}

		// Start runtime metrics collection
		if err := iruntime.Start(iruntime.WithMinimumReadMemStatsInterval(time.Second * 10)); err != nil {
			slog.Warn("failed to start runtime metrics", "error", err.Error())
		}

		// Start host metrics collection
		if err := host.Start(); err != nil {
			slog.Warn("failed to start host metrics", "error", err.Error())
		}

		shutdownFunc = func() error {
			defer cancel()
			slog.Info("Shutting down OpenTelemetry SDK")
			ctx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			return otelShutdown(ctx)
		}
		slog.Info("OTLP telemetry enabled",
			"service", serviceName,
			"endpoint", otelEndpoint,
			"insecure", os.Getenv("OTEL_INSECURE") == "true",
		)
	} else {
		// Configure slog without OpenTelemetry. Even without OTLP, the
		// TraceHandler still emits trace_id/span_id for any in-process spans
		// (e.g. otelhttp middleware spans created when SDK is initialized
		// with a no-op exporter), so log correlation tooling that ingests
		// stdout still works in dev.
		logger = slog.New(logging.NewTraceHandler(slog.NewTextHandler(os.Stdout, opts))).With(
			slog.String("service", serviceName),
		)
		slog.SetDefault(logger)
		shutdownFunc = func() error {
			defer cancel()
			return nil
		}
		slog.Info("OTLP telemetry disabled; logs go to stdout only",
			"service", serviceName,
			"reason", otelDisabledReason(otelEndpoint),
		)
	}

	return ctx, shutdownFunc, nil
}

func otelDisabledReason(endpoint string) string {
	switch {
	case os.Getenv("OTEL_SERVICE_NAME") == "" && endpoint == "":
		return "OTEL_SERVICE_NAME and OTEL_EXPORTER_OTLP_ENDPOINT are unset"
	case os.Getenv("OTEL_SERVICE_NAME") == "":
		return "OTEL_SERVICE_NAME is unset"
	case endpoint == "":
		return "OTEL_EXPORTER_OTLP_ENDPOINT is unset"
	default:
		return "unknown"
	}
}

// handleSignals sets up a context that will be cancelled when interrupt or termination signals are received
func handleSignals(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}

// setupOTelSDK bootstraps the OpenTelemetry pipeline with OTLP exporters
func setupOTelSDK(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	insecure := os.Getenv("OTEL_INSECURE") == "true"

	// shutdown calls cleanup functions registered via shutdownFuncs
	shutdown = func(ctx context.Context) error {
		slog.Debug("Running OpenTelemetry shutdown functions")
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		slog.Debug("OpenTelemetry shutdown complete")
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Set up propagator
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider
	tracerProvider, err := newTracerProvider(ctx, insecure)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Set up meter provider
	meterProvider, err := newMeterProvider(ctx, insecure)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	// Set up logger provider
	loggerProvider, err := newLoggerProvider(ctx, insecure)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	global.SetLoggerProvider(loggerProvider)

	return
}

// newPropagator creates a composite propagator for trace context and baggage
func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// newTracerProvider creates a new tracer provider with OTLP HTTP exporter
func newTracerProvider(ctx context.Context, insecure bool) (*sdktrace.TracerProvider, error) {
	opts := []otlptracehttp.Option{}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	traceExporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "griffin-commerce"
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(newResource(serviceName)),
	)
	return tracerProvider, nil
}

// newMeterProvider creates a new meter provider with OTLP HTTP exporter
func newMeterProvider(ctx context.Context, insecure bool) (*metric.MeterProvider, error) {
	opts := []otlpmetrichttp.Option{}
	if insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	metricExporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "griffin-commerce"
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(newResource(serviceName)),
	)
	return meterProvider, nil
}

// newLoggerProvider creates a new logger provider with OTLP HTTP exporter
func newLoggerProvider(ctx context.Context, insecure bool) (*otellog.LoggerProvider, error) {
	opts := []otlploghttp.Option{}
	if insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}
	logExporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "griffin-commerce"
	}

	loggerProvider := otellog.NewLoggerProvider(
		otellog.WithProcessor(otellog.NewBatchProcessor(logExporter)),
		otellog.WithResource(newResource(serviceName)),
	)
	return loggerProvider, nil
}

// newResource creates a resource for the service. Merges OTel's standard
// env-detected attributes (OTEL_RESOURCE_ATTRIBUTES, OTEL_SERVICE_NAME)
// so demos can stamp customer.id / service.namespace / etc. via env vars
// without code changes. Programmatic service.name + service.version
// override the env values to keep service identity stable.
func newResource(serviceName string) *resource.Resource {
	static := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion("1.0.0"),
	)
	envRes, err := resource.New(context.Background(), resource.WithFromEnv())
	if err != nil || envRes == nil {
		return static
	}
	// Merge order: env first, static second — static overrides on conflict.
	merged, mergeErr := resource.Merge(envRes, static)
	if mergeErr != nil {
		return static
	}
	return merged
}

// InitTelemetry is a legacy wrapper for backward compatibility
// Deprecated: Use SetupTelemetry instead
func InitTelemetry(serviceName string) (func(), error) {
	ctx, shutdown, err := SetupTelemetry(serviceName, nil)
	if err != nil {
		return nil, err
	}

	// Store the context for potential future use
	_ = ctx

	return func() {
		if err := shutdown(); err != nil {
			slog.Error("Error during telemetry shutdown", "error", err)
		}
	}, nil
}

// GetTracer returns a tracer for the given name
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
