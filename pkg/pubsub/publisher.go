package pubsub

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/ctrlaltcloud008-hub/prj-apex-core-modules/pkg/outbox"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Publisher struct {
	inner *pubsub.Publisher
	topic string
}

type PublisherOption func(*pubsub.Publisher)

func WithCountThreshold(count int) PublisherOption {
	return func(p *pubsub.Publisher) { p.PublishSettings.CountThreshold = count }
}

func WithByteThreshold(bytes int) PublisherOption {
	return func(p *pubsub.Publisher) { p.PublishSettings.ByteThreshold = bytes }
}

func WithDelayThreshold(delay time.Duration) PublisherOption {
	return func(p *pubsub.Publisher) { p.PublishSettings.DelayThreshold = delay }
}

func NewPublisher(client *pubsub.Client, topicID string, opts ...PublisherOption) *Publisher {

	pub := client.Publisher(topicID)
	for _, opt := range opts {
		opt(pub)
	}
	return &Publisher{
		inner: pub,
		topic: topicID,
	}
}

func (p *Publisher) Publish(ctx context.Context, msg *pubsub.Message) *pubsub.PublishResult {
	if msg.Attributes == nil {
		msg.Attributes = make(map[string]string)
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Attributes))
	return p.inner.Publish(ctx, msg)
}

func (p *Publisher) PublishFromOutbox(ctx context.Context, env outbox.Envelope, msg *pubsub.Message) *pubsub.PublishResult {
	if msg.Attributes == nil {
		msg.Attributes = make(map[string]string)
	}

	if env.Traceparent != "" {
		msg.Attributes["traceparent"] = env.Traceparent
	}
	if env.Tracestate != "" {
		msg.Attributes["tracestate"] = env.Tracestate
	}

	return p.inner.Publish(ctx, msg)
}

func (p *Publisher) Stop() { p.inner.Stop() }

func (p *Publisher) Topic() string { return p.topic }
