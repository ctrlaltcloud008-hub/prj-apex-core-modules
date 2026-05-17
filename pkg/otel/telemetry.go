package otel

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"
const googleTelemetryEndpoint = "telemetry.googleapis.com:443"

type TracerConfig struct {
	AppEnv      string
	ServiceName string
	ProjectID   string
	Region      string
}

const TracerName = "internal/otel"

func defaultServiceNamespace(projectID string) string {
	if projectID == "" {
		return "default"
	}

	return projectID
}

func defaultServiceInstanceID(serviceName string) string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}

	return fmt.Sprintf("%s-%s-%d", serviceName, hostname, os.Getpid())
}

func InitTracer(ctx context.Context, cfg TracerConfig) (func(ctx context.Context) error, error) {

	var shutdownFuncs []func(ctx context.Context) error
	exportCtx := context.WithoutCancel(ctx)

	shutdown := func(ctx context.Context) error {
		var err error
		for i := len(shutdownFuncs) - 1; i >= 0; i-- {
			err = errors.Join(err, shutdownFuncs[i](ctx))
		}
		shutdownFuncs = nil
		return err
	}

	cleanupOnError := func(err error) error {
		return errors.Join(err, shutdown(ctx))
	}

	prop := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

	otel.SetTextMapPropagator(prop)

	serviceNamespace := defaultServiceNamespace(cfg.ProjectID)
	serviceInstanceID := defaultServiceInstanceID(cfg.ServiceName)

	res, err := sdkresource.New(
		ctx,
		sdkresource.WithTelemetrySDK(),
		sdkresource.WithDetectors(gcp.NewDetector()),
		sdkresource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceNamespace(serviceNamespace),
			semconv.ServiceInstanceID(serviceInstanceID),
			semconv.CloudRegion(cfg.Region),
			attribute.String("service.environment", cfg.AppEnv),
			attribute.String("project.id", cfg.ProjectID),
			attribute.String("gcp.project_id", cfg.ProjectID),
		),
	)

	if err != nil {
		return shutdown, cleanupOnError(err)
	}

	if cfg.AppEnv == "local" {
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
			sdktrace.WithResource(res),
		)

		shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
		shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
		otel.SetMeterProvider(mp)
		otel.SetTracerProvider(tp)
		return shutdown, nil
	}

	creds, err := oauth.NewApplicationDefault(exportCtx, cloudPlatformScope)
	if err != nil {
		return shutdown, cleanupOnError(err)
	}

	exporter, err := otlptracegrpc.New(
		exportCtx,
		otlptracegrpc.WithEndpoint(googleTelemetryEndpoint),
		otlptracegrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)),
		otlptracegrpc.WithHeaders(map[string]string{
			"x-goog-user-project": cfg.ProjectID,
		}))

	if err != nil {
		return shutdown, cleanupOnError(err)
	}

	metricExporter, err := otlpmetricgrpc.New(
		exportCtx,
		otlpmetricgrpc.WithEndpoint(googleTelemetryEndpoint),
		otlpmetricgrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)),
		otlpmetricgrpc.WithHeaders(map[string]string{
			"x-goog-user-project": cfg.ProjectID,
		}),
	)
	if err != nil {
		return shutdown, cleanupOnError(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithBatcher(exporter), sdktrace.WithResource(res))
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetMeterProvider(mp)
	otel.SetTracerProvider(tp)

	return shutdown, nil
}
