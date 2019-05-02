package trace

import (
	"context"
	"math/rand"
	"time"

	"github.com/Microsoft/go-winio/pkg/etw"
)

func init() {
	// correlationvector uses math/rand, so need to seed it here.
	rand.Seed(time.Now().UnixNano())
}

var p *etw.Provider

// InitWithProvider initializes the trace library to use the given ETW provider.
func InitWithProvider(provider *etw.Provider) {
	p = provider
}

// NewSpan creates a new span based on the span from the given context.
func NewSpan(ctx context.Context, name string, baggage []Field) (context.Context, *Span) {
	return GetSpan(ctx).NewSpan(ctx, name, baggage)
}

type ctxKey struct{}

var spanKey = ctxKey{}

// GetSpan returns the span from the given context.
func GetSpan(ctx context.Context) *Span {
	if v := ctx.Value(spanKey); v != nil {
		return v.(*Span)
	}

	return &Span{}
}

// End ends the span from the given context.
func End(ctx context.Context, err error) {
	GetSpan(ctx).End(err)
}

// Error logs an error level event associated with the span from the given
// context.
func Error(ctx context.Context, name string, fields []Field) {
	GetSpan(ctx).Error(name, fields)
}

// Warn logs a warn level event associated with the span from the given context.
func Warn(ctx context.Context, name string, fields []Field) {
	GetSpan(ctx).Warn(name, fields)
}

// Info logs a info level event associated with the span from the given context.
func Info(ctx context.Context, name string, fields []Field) {
	GetSpan(ctx).Info(name, fields)
}

// Debug logs a debug level event associated with the span from the given
// context.
func Debug(ctx context.Context, name string, fields []Field) {
	GetSpan(ctx).Debug(name, fields)
}
