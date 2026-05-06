// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestTraceHandlerAttachesTraceIDFromContext verifies that a slog record
// emitted via the *Context variant gains trace_id and span_id attributes
// when the context carries a valid OpenTelemetry span.
//
// This is the load-bearing assertion for Phase 0: it proves that callers
// migrated to slog.*Context produce log lines that the side-drawer
// {trace_id="..."} query can correlate.
func TestTraceHandlerAttachesTraceIDFromContext(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(NewTraceHandler(inner))

	tp := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(tracetest.NewInMemoryExporter())))
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	wantTrace := span.SpanContext().TraceID().String()
	wantSpan := span.SpanContext().SpanID().String()

	logger.InfoContext(ctx, "hello", "k", "v")
	span.End()

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode log line: %v\nraw: %s", err, buf.String())
	}

	if got["trace_id"] != wantTrace {
		t.Errorf("trace_id = %v, want %s", got["trace_id"], wantTrace)
	}
	if got["span_id"] != wantSpan {
		t.Errorf("span_id = %v, want %s", got["span_id"], wantSpan)
	}
	if got["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", got["msg"])
	}
	if got["k"] != "v" {
		t.Errorf("preserved attr k = %v, want v", got["k"])
	}
}

// TestTraceHandlerNoSpanContext verifies that a slog record emitted with no
// active span passes through cleanly — no trace_id/span_id attrs are added.
// This matters for startup/shutdown logs that don't run inside a request.
func TestTraceHandlerNoSpanContext(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(NewTraceHandler(inner))

	logger.InfoContext(context.Background(), "startup", "port", 8080)

	out := buf.String()
	if strings.Contains(out, "trace_id") {
		t.Errorf("expected no trace_id attribute, got: %s", out)
	}
	if strings.Contains(out, "span_id") {
		t.Errorf("expected no span_id attribute, got: %s", out)
	}
}

// TestTraceHandlerBareCallNoContext verifies that bare slog.Info (no Context)
// also passes through without trace attributes — the slog API delivers a
// background context for those records, which has no span.
func TestTraceHandlerBareCallNoContext(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(NewTraceHandler(inner))

	logger.Info("bare call")

	out := buf.String()
	if strings.Contains(out, "trace_id") {
		t.Errorf("expected no trace_id on bare call, got: %s", out)
	}
}
