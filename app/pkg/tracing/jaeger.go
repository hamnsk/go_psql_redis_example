package tracing

import (
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics/prometheus"
	"io"
	"redis/pkg/logging"
)

func InitTracing(l *logging.Logger) (error, opentracing.Tracer, io.Closer) {
	tracingCfg, err := jaegercfg.FromEnv()

	if err != nil {
		l.Fatal("Could not parse Jaeger env vars: " + err.Error())
		return err, nil, nil
	}
	tracingCfg.ServiceName = "redis_cache_go_example"
	tracingCfg.Reporter.LogSpans = true
	tracingCfg.Sampler.Type = jaeger.SamplerTypeRemote
	tracingCfg.Sampler.Param = 1

	//tracingCfg := jaegercfg.Configuration{
	//	ServiceName: "redis_cache_go_example",
	//	Sampler: &jaegercfg.SamplerConfig{
	//		Type:  jaeger.SamplerTypeRemote,
	//		Param: 1,
	//	},
	//	Reporter: &jaegercfg.ReporterConfig{
	//		LogSpans: true,
	//	},
	//}

	jLogger := jaegerlog.StdLogger
	jMetricsFactory := prometheus.New()

	tracer, closer, err := tracingCfg.NewTracer(
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory),
	)

	if err != nil {
		l.Fatal("Init Jaeger failed: " + err.Error())
		return err, nil, nil
	}

	opentracing.SetGlobalTracer(tracer)
	return nil, tracer, closer
}
