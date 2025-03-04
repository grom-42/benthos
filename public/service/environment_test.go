package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benthosdev/benthos/v4/public/bloblang"
	"github.com/benthosdev/benthos/v4/public/service"
)

func walkForSummaries(fn func(func(name string, config *service.ConfigView))) map[string]string {
	summaries := map[string]string{}
	fn(func(name string, config *service.ConfigView) {
		summaries[name] = config.Summary()
	})
	return summaries
}

func TestEnvironmentAdjustments(t *testing.T) {
	envOne := service.NewEnvironment()
	envTwo := envOne.Clone()

	assert.NoError(t, envOne.RegisterCache(
		"one_cache", service.NewConfigSpec().Summary("cache one"),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Cache, error) {
			return nil, errors.New("cache one err")
		},
	))
	assert.NoError(t, envOne.RegisterInput(
		"one_input", service.NewConfigSpec().Summary("input one"),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			return nil, errors.New("input one err")
		},
	))
	assert.NoError(t, envOne.RegisterOutput(
		"one_output", service.NewConfigSpec().Summary("output one"),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Output, int, error) {
			return nil, 0, errors.New("output one err")
		},
	))
	assert.NoError(t, envOne.RegisterProcessor(
		"one_processor", service.NewConfigSpec().Summary("processor one"),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Processor, error) {
			return nil, errors.New("processor one err")
		},
	))
	assert.NoError(t, envOne.RegisterRateLimit(
		"one_rate_limit", service.NewConfigSpec().Summary("rate limit one"),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.RateLimit, error) {
			return nil, errors.New("rate limit one err")
		},
	))

	assert.Equal(t, "cache one", walkForSummaries(envOne.WalkCaches)["one_cache"])
	assert.Equal(t, "input one", walkForSummaries(envOne.WalkInputs)["one_input"])
	assert.Equal(t, "output one", walkForSummaries(envOne.WalkOutputs)["one_output"])
	assert.Equal(t, "processor one", walkForSummaries(envOne.WalkProcessors)["one_processor"])
	assert.Equal(t, "rate limit one", walkForSummaries(envOne.WalkRateLimits)["one_rate_limit"])

	assert.NotContains(t, walkForSummaries(envTwo.WalkCaches), "one_cache")
	assert.NotContains(t, walkForSummaries(envTwo.WalkInputs), "one_input")
	assert.NotContains(t, walkForSummaries(envTwo.WalkOutputs), "one_output")
	assert.NotContains(t, walkForSummaries(envTwo.WalkProcessors), "one_processor")
	assert.NotContains(t, walkForSummaries(envTwo.WalkRateLimits), "one_rate_limit")

	testConfig := `
input:
  one_input: {}
pipeline:
  processors:
    - one_processor: {}
output:
  one_output: {}
cache_resources:
  - label: foocache
    one_cache: {}
rate_limit_resources:
  - label: foorl
    one_rate_limit: {}
`

	assert.NoError(t, envOne.NewStreamBuilder().SetYAML(testConfig))
	assert.Error(t, envTwo.NewStreamBuilder().SetYAML(testConfig))
}

func TestEnvironmentBloblangIsolation(t *testing.T) {
	bEnv := bloblang.NewEnvironment().WithoutFunctions("now")
	require.NoError(t, bEnv.RegisterFunctionV2("meow", bloblang.NewPluginSpec(), func(args *bloblang.ParsedParams) (bloblang.Function, error) {
		return func() (interface{}, error) {
			return "meow", nil
		}, nil
	}))

	envOne := service.NewEnvironment()
	envOne.UseBloblangEnvironment(bEnv)

	badConfig := `
pipeline:
  processors:
    - bloblang: 'root = now()'
`

	goodConfig := `
pipeline:
  processors:
    - bloblang: 'root = meow()'

output:
  drop: {}

logger:
  level: OFF
`

	assert.Error(t, envOne.NewStreamBuilder().SetYAML(badConfig))

	strmBuilder := envOne.NewStreamBuilder()
	require.NoError(t, strmBuilder.SetYAML(goodConfig))

	var received []string
	require.NoError(t, strmBuilder.AddConsumerFunc(func(c context.Context, m *service.Message) error {
		b, err := m.AsBytes()
		if err != nil {
			return err
		}
		received = append(received, string(b))
		return nil
	}))

	pFn, err := strmBuilder.AddProducerFunc()
	require.NoError(t, err)

	strm, err := strmBuilder.Build()
	require.NoError(t, err)

	go func() {
		require.NoError(t, strm.Run(context.Background()))
	}()

	require.NoError(t, pFn(context.Background(), service.NewMessage([]byte("hello world"))))

	require.NoError(t, strm.StopWithin(time.Second))
	assert.Equal(t, []string{"meow"}, received)
}
