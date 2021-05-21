package internal

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8/internal/proto"
	"github.com/go-redis/redis/v8/internal/util"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func Sleep(ctx context.Context, dur time.Duration) error {
	_, span := StartSpan(ctx, "time.Sleep")
	defer span.End()

	t := time.NewTimer(dur)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ToLower(s string) string {
	if isLower(s) {
		return s
	}

	b := make([]byte, len(s))
	for i := range b {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return util.BytesToString(b)
}

func isLower(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			return false
		}
	}
	return true
}

//------------------------------------------------------------------------------

var tracer = otel.Tracer("github.com/go-redis/redis")

func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	if span := trace.SpanFromContext(ctx); !span.IsRecording() {
		return ctx, span
	}
	return tracer.Start(ctx, name)
}

func RecordError(ctx context.Context, span trace.Span, err error) error {
	if err != proto.Nil {
		span.RecordError(err)
	}
	return err
}
