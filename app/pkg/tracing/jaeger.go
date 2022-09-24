package tracing

import (
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"os"
	"redis/pkg/logging"
)

func InitTracing(l *logging.Logger) (error, *tracesdk.TracerProvider) {
	host := os.Getenv("JAEGER_AGENT_HOST")
	port := os.Getenv("JAEGER_AGENT_PORT")
	if host == "" || port == "" {
		l.Fatal("Could not parse Jaeger env vars. Please set JAEGER_AGENT_HOST & JAEGER_AGENT_PORT ")
	}

	tp, err := tracerProvider(fmt.Sprintf("http://%s:%s/api/traces", host, port),
		"redis_cache_go_example",
		"prod",
		1,
	)
	if err != nil {
		l.Fatal("ERROR: cannot init Jaeger: " + err.Error())
		return err, nil
	}

	return nil, tp
}

func tracerProvider(url, service, environment string, id int64) (*tracesdk.TracerProvider, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
			attribute.String("environment", environment),
			attribute.Int64("ID", id),
		)),
	)
	return tp, nil
}
