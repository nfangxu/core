package otmongo

import (
	"context"
	"fmt"
	"os"

	"github.com/DoNewsCode/core/config"
	"github.com/DoNewsCode/core/contract"
	"github.com/DoNewsCode/core/di"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/opentracing/opentracing-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/dig"
)

/*
Providers returns a set of dependency providers. It includes the Maker, the
default mongo.Client and exported configs.
	Depends On:
		log.Logger
		contract.ConfigAccessor
		MongoConfigInterceptor `optional:"true"`
		opentracing.Tracer     `optional:"true"`
	Provides:
		Factory
		Maker
		*mongo.Client
*/
func Providers() di.Deps {
	return []interface{}{provideMongoFactory, provideDefaultClient, provideConfig}
}

// MongoConfigInterceptor is an injection type hint that allows user to make last
// minute modification to mongo configuration. This is useful when some
// configuration cannot be easily expressed in a text form. For example, the
// options.ContextDialer.
type MongoConfigInterceptor func(name string, clientOptions *options.ClientOptions)

// Maker models Factory
type Maker interface {
	Make(name string) (*mongo.Client, error)
}

// Factory is a *di.Factory that creates *mongo.Client using a specific
// configuration entry.
type Factory struct {
	*di.Factory
}

// Make creates *mongo.Client using a specific configuration entry.
func (r Factory) Make(name string) (*mongo.Client, error) {
	client, err := r.Factory.Make(name)
	if err != nil {
		return nil, err
	}
	return client.(*mongo.Client), nil
}

// in is the injection parameter for Provide.
type in struct {
	dig.In

	Logger      log.Logger
	Conf        contract.ConfigAccessor
	Interceptor MongoConfigInterceptor `optional:"true"`
	Tracer      opentracing.Tracer     `optional:"true"`
}

// out is the result of Provide. The official mongo package doesn't
// provide a proper interface type. It is up to the users to define their own
// mongodb repository interface.
type out struct {
	dig.Out

	Factory Factory
	Maker   Maker
}

// Provide creates Factory and *mongo.Client. It is a valid dependency for
// package core.
func provideMongoFactory(p in) (out, func()) {
	var err error
	var dbConfs map[string]struct{ Uri string }
	err = p.Conf.Unmarshal("mongo", &dbConfs)
	if err != nil {
		level.Warn(p.Logger).Log("err", err)
	}
	factory := di.NewFactory(func(name string) (di.Pair, error) {
		var (
			ok   bool
			conf struct{ Uri string }
		)
		if conf, ok = dbConfs[name]; !ok {
			if name != "default" {
				return di.Pair{}, fmt.Errorf("mongo configuration %s not valid", name)
			}
			conf.Uri = "mongodb://127.0.0.1:27017"
			if os.Getenv("MONGO_ADDR") != "" {
				conf.Uri = os.Getenv("MONGO_ADDR")
			}
		}
		opts := options.Client()
		opts.ApplyURI(conf.Uri)
		if p.Tracer != nil {
			opts.Monitor = NewMonitor(p.Tracer)
		}
		if p.Interceptor != nil {
			p.Interceptor(name, opts)
		}
		client, err := mongo.Connect(context.Background(), opts)
		if err != nil {
			return di.Pair{}, err
		}
		return di.Pair{
			Conn: client,
			Closer: func() {
				_ = client.Disconnect(context.Background())
			},
		}, nil
	})
	f := Factory{factory}
	return out{
		Factory: f,
		Maker:   f,
	}, factory.Close
}

func provideDefaultClient(maker Maker) (*mongo.Client, error) {
	return maker.Make("default")
}

type configOut struct {
	di.Out

	Config []config.ExportedConfig `group:"config,flatten"`
}

// provideConfig exports the default mongo configuration.
func provideConfig() configOut {
	configs := []config.ExportedConfig{
		{
			Owner: "otmongo",
			Data: map[string]interface{}{
				"mongo": map[string]struct {
					Uri string `json:"uri" yaml:"uri"`
				}{
					"default": {
						Uri: "",
					},
				},
			},
			Comment: "The configuration of mongoDB",
		},
	}
	return configOut{Config: configs}
}
