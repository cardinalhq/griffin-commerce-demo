// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package orderevents

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

const logIntervalSeconds = 5

// StartLogEmitter launches a goroutine that emits log events on a fixed
// cadence until ctx is cancelled. Event frequency is gated on IsActive, the
// exact same time-window check that drives the metric ramp in metrics.go —
// so logs start and stop in lockstep with the lag spike by construction,
// not by manual tuning.
func StartLogEmitter(ctx context.Context) {
	logger := global.GetLoggerProvider().Logger(instrumentScope)
	interval := logIntervalSeconds * time.Second
	rng := rand.New(rand.NewSource(0x0eae172 ^ time.Now().UnixNano()))

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		wasActive := false
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				active := IsActive(now)
				emitTick(ctx, logger, now, active, rng)
				if wasActive && !active {
					emitRecord(ctx, logger, log.SeverityInfo, "INFO",
						"Consumer lag recovered for order.created, consumer group order-events-analyzer",
						log.String("source", "order-events-analyzer"),
						log.String("event_type", "order_events_lag_recovered"),
						log.String("topic", topic),
						log.String("consumer_group", consumerGroup),
					)
				}
				wasActive = active
			}
		}
	}()
}

func emitTick(ctx context.Context, logger log.Logger, now time.Time, active bool, rng *rand.Rand) {
	intervalMin := float64(logIntervalSeconds) / 60.0
	if !active {
		return
	}
	emitN(rng, 12*intervalMin, func() { logLagIncreasing(ctx, logger, now, rng) })
	emitN(rng, 6*intervalMin, func() { logDeadlineExceeded(ctx, logger, now, rng) })
}

// emitN samples a Poisson count from lambda and invokes fn that many times.
// Bounded to 50 emissions per tick per event so a runaway lambda can't flood.
func emitN(rng *rand.Rand, lambda float64, fn func()) {
	if lambda <= 0 {
		return
	}
	n := poisson(rng, lambda)
	if n > 50 {
		n = 50
	}
	for i := 0; i < n; i++ {
		fn()
	}
}

func poisson(rng *rand.Rand, lambda float64) int {
	if lambda < 30 {
		l := math.Exp(-lambda)
		p := 1.0
		k := 0
		for {
			k++
			p *= rng.Float64()
			if p <= l {
				return k - 1
			}
		}
	}
	return int(math.Round(lambda + rng.NormFloat64()*math.Sqrt(lambda)))
}

func emitRecord(ctx context.Context, logger log.Logger, sev log.Severity, sevText, body string, attrs ...log.KeyValue) {
	rec := log.Record{}
	rec.SetTimestamp(time.Now())
	rec.SetSeverity(sev)
	rec.SetSeverityText(sevText)
	rec.SetBody(log.StringValue(body))
	rec.AddAttributes(attrs...)
	logger.Emit(ctx, rec)
}

func logLagIncreasing(ctx context.Context, logger log.Logger, now time.Time, rng *rand.Rand) {
	lag := RampedValue(baselineLag, incidentLag, seed, now)
	partition := rng.Intn(3)
	msg := fmt.Sprintf("Consumer lag increasing on topic %s partition %d: lag_seconds=%.0f", topic, partition, lag)
	emitRecord(ctx, logger, log.SeverityWarn, "WARN", msg,
		log.String("source", "order-events-analyzer"),
		log.String("event_type", "order_events_lag_increasing"),
		log.String("topic", topic),
		log.String("consumer_group", consumerGroup),
		log.Int("partition", partition),
		log.Float64("lag_seconds", lag),
	)
}

func logDeadlineExceeded(ctx context.Context, logger log.Logger, now time.Time, rng *rand.Rand) {
	lag := RampedValue(baselineLag, incidentLag, seed, now)
	orderID := fmt.Sprintf("ord-%06d", rng.Intn(999999))
	msg := fmt.Sprintf("Event processing deadline exceeded for order.created event order_id=%s, consumer group %s", orderID, consumerGroup)
	emitRecord(ctx, logger, log.SeverityError, "ERROR", msg,
		log.String("source", "order-events-analyzer"),
		log.String("event_type", "order_events_processing_deadline_exceeded"),
		log.String("topic", topic),
		log.String("consumer_group", consumerGroup),
		log.String("order_id", orderID),
		log.Float64("lag_seconds", lag),
	)
}
