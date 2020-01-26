package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	logTracer "github.com/opentracing/opentracing-go/log"
	"github.com/uber/jaeger-client-go/config"
)

func main() {
	tracer, closer := initJeager()
	defer func() {
		err := closer.Close()
		if err != nil {
			fmt.Println(fmt.Sprintf("error while close tracer, got: %v", err))
		}
	}()

	fmt.Println("starting collector service")

	http.HandleFunc("/collect", func(writer http.ResponseWriter, request *http.Request) {
		spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(request.Header))
		span := tracer.StartSpan("collecting", ext.RPCServerOption(spanCtx))
		defer span.Finish()

		fmt.Println("collecting...")
		data := span.BaggageItem("data")
		if data != "" {
			fmt.Println(fmt.Sprintf("data valid. data: %s", data))
		}
		time.Sleep(2 * time.Second)
		fmt.Println("collected")
		fmt.Println("-------------")

		span.SetTag("collector", "mongodb")
		span.LogFields(
			logTracer.String("status", "success"),
		)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initJeager() (opentracing.Tracer, io.Closer) {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
		},
		ServiceName: "service-collector",
	}

	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		panic(fmt.Sprintf("error while create intance for tracer, got: %v", err))
	}

	return tracer, closer
}
