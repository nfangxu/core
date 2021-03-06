package otgorm

import (
	"testing"

	"github.com/DoNewsCode/core"
	"github.com/DoNewsCode/core/di"
	"gorm.io/gorm"

	"github.com/DoNewsCode/core/config"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
)

func TestProvideDBFactory(t *testing.T) {
	factory, cleanup := provideDBFactory(databaseIn{
		Conf: config.MapAdapter{"gorm": map[string]databaseConf{
			"default": {
				Database: "sqlite",
				Dsn:      ":memory:",
			},
			"alternative": {
				Database: "mysql",
				Dsn:      "root@tcp(127.0.0.1:3306)/app?charset=utf8mb4&parseTime=True&loc=Local",
			},
		}},
		Logger: log.NewNopLogger(),
		Tracer: nil,
	})
	alt, err := factory.Make("alternative")
	assert.NoError(t, err)
	assert.NotNil(t, alt)
	def, err := factory.Make("default")
	assert.NoError(t, err)
	assert.NotNil(t, def)
	cleanup()
}

func TestGorm(t *testing.T) {
	c := core.New()
	c.ProvideEssentials()
	c.Provide(Providers())
	c.Invoke(func(
		d1 Maker,
		d2 Factory,
		d3 struct {
			di.In
			Cfg []config.ExportedConfig `group:"config"`
		},
		d4 *gorm.DB,
	) {
		a := assert.New(t)
		a.NotNil(d1)
		a.NotNil(d2)
		a.NotEmpty(d3.Cfg)
		a.NotNil(d4)
	})
}

func TestProvideMemoryDatabase(t *testing.T) {
	c := core.New()
	c.ProvideEssentials()
	c.Provide(di.Deps{provideMemoryDatabase})
	c.Invoke(func(d1 *SQLite) {
		assert.Equal(t, "sqlite", d1.Name())
	})
}

func TestProvideConfigs(t *testing.T) {
	c := provideConfig()
	assert.NotEmpty(t, c.Config)
}
