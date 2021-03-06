package otes

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/DoNewsCode/core/config"
	"github.com/go-kit/kit/log"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
)

func TestTracing(t *testing.T) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)
	factory, cleanup := provideEsFactory(in{
		Conf: config.MapAdapter{"es": map[string]Config{
			"default":     {URL: []string{"http://localhost:9200"}},
			"alternative": {URL: []string{"http://localhost:9200"}},
		}},
		Logger: log.NewNopLogger(),
		Tracer: tracer,
	})
	defer cleanup()

	client, err := factory.Maker.Make("default")
	assert.NoError(t, err)
	span, ctx := opentracing.StartSpanFromContextWithTracer(context.Background(), tracer, "es.query")
	defer span.Finish()

	res, code, err := client.Ping(os.Getenv("ELASTICSEARCH_ADDR")).Do(ctx)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.NotNil(t, res)
	assert.NotEmpty(t, tracer.FinishedSpans())
}
