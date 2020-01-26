package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/uber/jaeger-client-go/config"
)

type Order struct {
	ID   int
	Name string
}

func main() {
	tracer, closer := initJeager()
	defer func() {
		err := closer.Close()
		if err != nil {
			fmt.Println(fmt.Sprintf("error while close tracer, got: %v", err))
		}
	}()

	id := orderIDGenerator()
	span := tracer.StartSpan("svc.order")
	defer span.Finish()
	opentracing.SetGlobalTracer(tracer)

	orderData := Order{}
	orderData.ID = id
	orderData.Name = "author"

	span.SetBaggageItem("data", fmt.Sprint(orderData))
	span.SetTag("svc.order", fmt.Sprintf("order-%d", id))
	span.LogFields(
		log.String("event", "order"),
		log.String("id", fmt.Sprint(id)),
	)

	ctx := opentracing.ContextWithSpan(context.Background(), span)
	storeOrder(ctx, id)
	updateOrder(ctx)
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
		ServiceName: "order.service-connector",
	}

	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		panic(fmt.Sprintf("error while create intance for tracer, got: %v", err))
	}

	return tracer, closer
}

func orderIDGenerator() int {
	rand.Seed(time.Now().UnixNano())
	min := 10
	max := 30
	return rand.Intn(max-min+1) + min
}

func storeOrder(ctx context.Context, id int) {
	span, _ := opentracing.StartSpanFromContext(ctx, "storing.order")
	defer span.Finish()

	url := "http://localhost:8080/collect"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}

	ext.SpanKindRPCClient.Set(span)
	ext.HTTPUrl.Set(span, url)
	ext.HTTPMethod.Set(span, http.MethodGet)
	err = span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		fmt.Println(fmt.Sprintf("error while inject span into http header: %v", err))
	}

	_, err = Do(req)
	if err != nil {
		panic(err.Error())
	}

	span.SetTag("store", id)
	span.LogFields(
		log.String("event", "store"),
		log.String("id", fmt.Sprint(id)),
	)
}

func updateOrder(ctx context.Context) {
	span, _ := opentracing.StartSpanFromContext(ctx, "update.order")
	defer span.Finish()

	fmt.Println("processing...")
	time.Sleep(3 * time.Second)
	fmt.Println("process is done")

	span.SetTag("update.order", "static")
}

func Do(req *http.Request) ([]byte, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			fmt.Println(fmt.Errorf("error close response body, got: %v", err))
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http status code: %d, body: %s", resp.StatusCode, body)
	}

	return body, nil
}
