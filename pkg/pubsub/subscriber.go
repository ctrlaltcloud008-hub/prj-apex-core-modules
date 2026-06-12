package pubsub

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type SubscriberOption func(*pubsub.Subscriber)

func WithMaxOutstandingMessages(max int) SubscriberOption {
	return func(s *pubsub.Subscriber) { s.ReceiveSettings.MaxOutstandingMessages = max }
}

func WithNumGoroutines(num int) SubscriberOption {
	return func(s *pubsub.Subscriber) { s.ReceiveSettings.NumGoroutines = num }
}

func WithMaxOutstandingBytes(max int) SubscriberOption {
	return func(s *pubsub.Subscriber) { s.ReceiveSettings.MaxOutstandingBytes = max }
}

// WithMaxExtension sets how long the client keeps extending the ack deadline of
// a message being processed. Must exceed the longest expected handler duration
// (e.g. a multi-hour FFmpeg encode); the library default is 60 minutes.
func WithMaxExtension(d time.Duration) SubscriberOption {
	return func(s *pubsub.Subscriber) { s.ReceiveSettings.MaxExtension = d }
}

// WithMinExtensionPeriod sets the minimum ack-deadline extension period,
// reducing ModifyAckDeadline RPC churn for long-running handlers.
func WithMinExtensionPeriod(d time.Duration) SubscriberOption {
	return func(s *pubsub.Subscriber) { s.ReceiveSettings.MinDurationPerAckExtension = d }
}

type Subscriber struct {
	inner *pubsub.Subscriber
	sub   string
}

func NewSubscriber(client *pubsub.Client, subscriptionID string, opts ...SubscriberOption) *Subscriber {
	sub := client.Subscriber(subscriptionID)
	for _, opt := range opts {
		opt(sub)
	}
	return &Subscriber{inner: sub, sub: subscriptionID}
}

func (s *Subscriber) Receive(ctx context.Context, f func(ctx context.Context, msg *pubsub.Message)) error {
	return s.inner.Receive(ctx, f)
}

func (s *Subscriber) Subscription() string {
	return s.sub
}

func StartConsumerSpan(ctx context.Context, msg *pubsub.Message, spanName string) (context.Context, trace.Span) {
	attrs := msg.Attributes
	if attrs == nil {
		attrs = map[string]string{}
	}

	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(attrs))
	ctx, span := otel.Tracer("internal/pubsub").Start(
		ctx,
		spanName,
		trace.WithSpanKind(trace.SpanKindConsumer),
	)

	if videoID := attrs["video.id"]; videoID != "" {
		span.SetAttributes(attribute.String("video.id", videoID))
	} else if videoID := attrs["video_id"]; videoID != "" {
		span.SetAttributes(attribute.String("video.id", videoID))
	}

	return ctx, span
}
