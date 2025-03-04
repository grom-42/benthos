package service_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benthosdev/benthos/v4/public/service"

	_ "github.com/benthosdev/benthos/v4/public/components/all"
)

func TestStreamBuilderDefault(t *testing.T) {
	b := service.NewStreamBuilder()

	act, err := b.AsYAML()
	require.NoError(t, err)

	exp := []string{
		`input:
    label: ""
    stdin:`,
		`buffer:
    none: {}`,
		`pipeline:
    threads: 0
    processors: []`,
		`output:
    label: ""
    stdout:`,
		`logger:
    level: INFO`,
		`metrics:
    prometheus:`,
	}

	for _, str := range exp {
		assert.Contains(t, act, str)
	}
}

func TestStreamBuilderProducerFunc(t *testing.T) {
	tmpDir := t.TempDir()

	outFilePath := filepath.Join(tmpDir, "out.txt")

	b := service.NewStreamBuilder()
	require.NoError(t, b.SetLoggerYAML("level: NONE"))
	require.NoError(t, b.AddProcessorYAML(`bloblang: 'root = content().uppercase()'`))
	require.NoError(t, b.AddOutputYAML(fmt.Sprintf(`
file:
  codec: lines
  path: %v`, outFilePath)))

	pushFn, err := b.AddProducerFunc()
	require.NoError(t, err)

	// Fails on second call.
	_, err = b.AddProducerFunc()
	require.Error(t, err)

	// Don't allow input overrides now.
	err = b.SetYAML(`input: {}`)
	require.Error(t, err)

	strm, err := b.Build()
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx, done := context.WithTimeout(context.Background(), time.Second*10)
		defer done()

		require.NoError(t, pushFn(ctx, service.NewMessage([]byte("hello world 1"))))
		require.NoError(t, pushFn(ctx, service.NewMessage([]byte("hello world 2"))))
		require.NoError(t, pushFn(ctx, service.NewMessage([]byte("hello world 3"))))

		require.NoError(t, strm.StopWithin(time.Second*5))
	}()

	require.NoError(t, strm.Run(context.Background()))
	wg.Wait()

	outBytes, err := os.ReadFile(outFilePath)
	require.NoError(t, err)

	assert.Equal(t, "HELLO WORLD 1\nHELLO WORLD 2\nHELLO WORLD 3\n", string(outBytes))
}

func TestStreamBuilderBatchProducerFunc(t *testing.T) {
	tmpDir := t.TempDir()

	outFilePath := filepath.Join(tmpDir, "out.txt")

	b := service.NewStreamBuilder()
	require.NoError(t, b.SetLoggerYAML("level: NONE"))
	require.NoError(t, b.AddProcessorYAML(`bloblang: 'root = content().uppercase()'`))
	require.NoError(t, b.AddOutputYAML(fmt.Sprintf(`
file:
  codec: lines
  path: %v`, outFilePath)))

	pushFn, err := b.AddBatchProducerFunc()
	require.NoError(t, err)

	// Fails on second call.
	_, err = b.AddProducerFunc()
	require.Error(t, err)

	// Don't allow input overrides now.
	err = b.SetYAML(`input: {}`)
	require.Error(t, err)

	strm, err := b.Build()
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx, done := context.WithTimeout(context.Background(), time.Second*10)
		defer done()

		require.NoError(t, pushFn(ctx, service.MessageBatch{
			service.NewMessage([]byte("hello world 1")),
			service.NewMessage([]byte("hello world 2")),
		}))
		require.NoError(t, pushFn(ctx, service.MessageBatch{
			service.NewMessage([]byte("hello world 3")),
			service.NewMessage([]byte("hello world 4")),
		}))
		require.NoError(t, pushFn(ctx, service.MessageBatch{
			service.NewMessage([]byte("hello world 5")),
			service.NewMessage([]byte("hello world 6")),
		}))

		require.NoError(t, strm.StopWithin(time.Second*5))
	}()

	require.NoError(t, strm.Run(context.Background()))
	wg.Wait()

	outBytes, err := os.ReadFile(outFilePath)
	require.NoError(t, err)

	assert.Equal(t, "HELLO WORLD 1\nHELLO WORLD 2\n\nHELLO WORLD 3\nHELLO WORLD 4\n\nHELLO WORLD 5\nHELLO WORLD 6\n\n", string(outBytes))
}

func TestStreamBuilderEnvVarInterpolation(t *testing.T) {
	os.Setenv("BENTHOS_TEST_ONE", "foo")
	os.Setenv("BENTHOS_TEST_TWO", "bar")

	b := service.NewStreamBuilder()
	require.NoError(t, b.AddInputYAML(`
kafka:
  topics: [ ${BENTHOS_TEST_ONE} ]
`))

	require.NoError(t, b.SetLoggerYAML(`level: ${BENTHOS_TEST_TWO}`))

	act, err := b.AsYAML()
	require.NoError(t, err)

	exp := []string{
		` topics:
            - foo`,
		`level: bar`,
	}

	for _, str := range exp {
		assert.Contains(t, act, str)
	}

	b = service.NewStreamBuilder()
	require.NoError(t, b.SetYAML(`
input:
  kafka:
    topics: [ ${BENTHOS_TEST_ONE} ]
logger:
  level: ${BENTHOS_TEST_TWO}
`))

	act, err = b.AsYAML()
	require.NoError(t, err)

	for _, str := range exp {
		assert.Contains(t, act, str)
	}
}

func TestStreamBuilderConsumerFunc(t *testing.T) {
	tmpDir := t.TempDir()

	inFilePath := filepath.Join(tmpDir, "in.txt")
	require.NoError(t, os.WriteFile(inFilePath, []byte(`HELLO WORLD 1
HELLO WORLD 2
HELLO WORLD 3`), 0o755))

	b := service.NewStreamBuilder()
	require.NoError(t, b.SetLoggerYAML("level: NONE"))
	require.NoError(t, b.AddInputYAML(fmt.Sprintf(`
file:
  codec: lines
  paths: [ %v ]`, inFilePath)))
	require.NoError(t, b.AddProcessorYAML(`bloblang: 'root = content().lowercase()'`))

	outMsgs := map[string]struct{}{}
	var outMut sync.Mutex
	handler := func(_ context.Context, m *service.Message) error {
		outMut.Lock()
		defer outMut.Unlock()

		b, err := m.AsBytes()
		assert.NoError(t, err)

		outMsgs[string(b)] = struct{}{}
		return nil
	}
	require.NoError(t, b.AddConsumerFunc(handler))

	// Fails on second call.
	require.Error(t, b.AddConsumerFunc(handler))

	// Don't allow output overrides now.
	err := b.SetYAML(`output: {}`)
	require.Error(t, err)

	strm, err := b.Build()
	require.NoError(t, err)

	require.NoError(t, strm.Run(context.Background()))

	outMut.Lock()
	assert.Equal(t, map[string]struct{}{
		"hello world 1": {},
		"hello world 2": {},
		"hello world 3": {},
	}, outMsgs)
	outMut.Unlock()
}

func TestStreamBuilderBatchConsumerFunc(t *testing.T) {
	tmpDir := t.TempDir()

	inFilePath := filepath.Join(tmpDir, "in.txt")
	require.NoError(t, os.WriteFile(inFilePath, []byte(`HELLO WORLD 1
HELLO WORLD 2

HELLO WORLD 3
HELLO WORLD 4

HELLO WORLD 5
HELLO WORLD 6
`), 0o755))

	b := service.NewStreamBuilder()
	require.NoError(t, b.SetLoggerYAML("level: NONE"))
	require.NoError(t, b.AddInputYAML(fmt.Sprintf(`
file:
  codec: lines/multipart
  paths: [ %v ]`, inFilePath)))
	require.NoError(t, b.AddProcessorYAML(`bloblang: 'root = content().lowercase()'`))

	outBatches := map[string]struct{}{}
	var outMut sync.Mutex
	handler := func(_ context.Context, mb service.MessageBatch) error {
		outMut.Lock()
		defer outMut.Unlock()

		outMsgs := []string{}
		for _, m := range mb {
			b, err := m.AsBytes()
			assert.NoError(t, err)
			outMsgs = append(outMsgs, string(b))
		}

		outBatches[strings.Join(outMsgs, ",")] = struct{}{}
		return nil
	}
	require.NoError(t, b.AddBatchConsumerFunc(handler))

	// Fails on second call.
	require.Error(t, b.AddBatchConsumerFunc(handler))

	// Don't allow output overrides now.
	err := b.SetYAML(`output: {}`)
	require.Error(t, err)

	strm, err := b.Build()
	require.NoError(t, err)

	require.NoError(t, strm.Run(context.Background()))

	outMut.Lock()
	assert.Equal(t, map[string]struct{}{
		"hello world 1,hello world 2": {},
		"hello world 3,hello world 4": {},
		"hello world 5,hello world 6": {},
	}, outBatches)
	outMut.Unlock()
}

func TestStreamBuilderCustomLogger(t *testing.T) {
	b := service.NewStreamBuilder()
	b.SetPrintLogger(nil)

	act, err := b.AsYAML()
	require.NoError(t, err)

	exp := `logger:
    level: INFO`

	assert.NotContains(t, act, exp)
}

func TestStreamBuilderSetYAML(t *testing.T) {
	b := service.NewStreamBuilder()
	b.SetThreads(10)
	require.NoError(t, b.AddCacheYAML(`label: foocache
type: memory`))
	require.NoError(t, b.AddInputYAML(`type: kafka`))
	require.NoError(t, b.AddOutputYAML(`type: nats`))
	require.NoError(t, b.AddProcessorYAML(`type: bloblang`))
	require.NoError(t, b.AddProcessorYAML(`type: jmespath`))
	require.NoError(t, b.AddRateLimitYAML(`label: foorl
type: local`))
	require.NoError(t, b.SetMetricsYAML(`type: prometheus`))
	require.NoError(t, b.SetLoggerYAML(`level: DEBUG`))
	require.NoError(t, b.SetBufferYAML(`type: memory`))

	act, err := b.AsYAML()
	require.NoError(t, err)

	exp := []string{
		`input:
    label: ""
    kafka:`,
		`buffer:
    memory: {}`,
		`pipeline:
    threads: 10
    processors:`,
		`
        - label: ""
          bloblang: ""`,
		`
        - label: ""
          jmespath:
            query: ""`,
		`output:
    label: ""
    nats:`,
		`metrics:
    prometheus:`,
		`cache_resources:
    - label: foocache
      memory:`,
		`rate_limit_resources:
    - label: foorl
      local:`,
		`  level: DEBUG`,
	}

	for _, str := range exp {
		assert.Contains(t, act, str)
	}
}

func TestStreamBuilderSetResourcesYAML(t *testing.T) {
	b := service.NewStreamBuilder()
	require.NoError(t, b.AddResourcesYAML(`
cache_resources:
  - label: foocache
    type: memory

rate_limit_resources:
  - label: foorl
    type: local

processor_resources:
  - label: fooproc1
    type: bloblang
  - label: fooproc2
    type: jmespath

input_resources:
  - label: fooinput
    type: kafka

output_resources:
  - label: foooutput
    type: nats
`))

	act, err := b.AsYAML()
	require.NoError(t, err)

	exp := []string{
		`cache_resources:
    - label: foocache
      memory:`,
		`rate_limit_resources:
    - label: foorl
      local:`,
		`processor_resources:
    - label: fooproc1
      bloblang:`,
		`    - label: fooproc2
      jmespath:`,
		`input_resources:
    - label: fooinput
      kafka:`,
		`output_resources:
    - label: foooutput
      nats:`,
	}

	for _, str := range exp {
		assert.Contains(t, act, str)
	}
}

func TestStreamBuilderSetYAMLBrokers(t *testing.T) {
	b := service.NewStreamBuilder()
	b.SetThreads(10)
	require.NoError(t, b.AddInputYAML(`type: kafka`))
	require.NoError(t, b.AddInputYAML(`type: amqp_0_9`))
	require.NoError(t, b.AddOutputYAML(`type: nats`))
	require.NoError(t, b.AddOutputYAML(`type: file`))

	act, err := b.AsYAML()
	require.NoError(t, err)

	exp := []string{
		`input:
    label: ""
    broker:
        copies: 1
        inputs:`,
		`            - label: ""
              kafka:`,
		`            - label: ""
              amqp_0_9:`,
		`output:
    label: ""
    broker:
        copies: 1
        pattern: fan_out
        outputs:`,
		`            - label: ""
              nats:`,
		`            - label: ""
              file:`,
	}

	for _, str := range exp {
		assert.Contains(t, act, str)
	}
}

func TestStreamBuilderYAMLErrors(t *testing.T) {
	b := service.NewStreamBuilder()

	err := b.AddCacheYAML(`{ label: "", type: memory }`)
	require.Error(t, err)
	assert.EqualError(t, err, "a label must be specified for cache resources")

	err = b.AddInputYAML(`not valid ! yaml 34324`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal errors")

	err = b.SetYAML(`not valid ! yaml 34324`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected object value")

	err = b.SetYAML(`input: { foo: nope }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to infer")

	err = b.SetYAML(`input: { kafka: { not_a_field: nope } }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field not_a_field not recognised")

	err = b.AddInputYAML(`not_a_field: nah`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to infer")

	err = b.AddInputYAML(`kafka: { not_a_field: nah }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field not_a_field not recognised")

	err = b.SetLoggerYAML(`not_a_field: nah`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field not_a_field not recognised")

	err = b.AddRateLimitYAML(`{ label: "", local: {} }`)
	require.Error(t, err)
	assert.EqualError(t, err, "a label must be specified for rate limit resources")
}

func TestStreamBuilderSetFields(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		args        []interface{}
		output      string
		errContains string
	}{
		{
			name:  "odd number of args",
			input: `{}`,
			args: []interface{}{
				"just a field",
			},
			errContains: "odd number of pathValues",
		},
		{
			name:  "a path isnt a string",
			input: `{}`,
			args: []interface{}{
				10, "hello world",
			},
			errContains: "should be a string",
		},
		{
			name: "unknown field error",
			input: `
input:
  kafka:
    topics: [ foo, bar ]
`,
			args: []interface{}{
				"input.kafka.unknown_field", "baz",
			},
			errContains: "field not recognised",
		},
		{
			name: "create lint error",
			input: `
input:
  kafka:
    topics: [ foo, bar ]
`,
			args: []interface{}{
				"input.label", "foo",
				"output.label", "foo",
			},
			errContains: "collides with a previously",
		},
		{
			name: "set kafka input topics",
			input: `
input:
  kafka:
    topics: [ foo, bar ]
`,
			args: []interface{}{
				"input.kafka.topics.1", "baz",
			},
			output: `
input:
  kafka:
    topics: [ foo, baz ]
`,
		},
		{
			name: "append kafka input topics",
			input: `
input:
  kafka:
    topics: [ foo, bar ]
`,
			args: []interface{}{
				"input.kafka.topics.-", "baz",
				"input.kafka.topics.-", "buz",
				"input.kafka.topics.-", "bev",
			},
			output: `
input:
  kafka:
    topics: [ foo, bar, baz, buz, bev ]
`,
		},
		{
			name: "add a processor",
			input: `
input:
  kafka:
    topics: [ foo, bar ]
`,
			args: []interface{}{
				"pipeline.processors.-.bloblang", `root = "meow"`,
			},
			output: `
input:
  kafka:
    topics: [ foo, bar ]
pipeline:
  processors:
    - bloblang: 'root = "meow"'
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := service.NewStreamBuilder()
			require.NoError(t, b.SetYAML(test.input))
			err := b.SetFields(test.args...)
			if test.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.errContains)
			} else {
				require.NoError(t, err)

				b2 := service.NewStreamBuilder()
				require.NoError(t, b2.SetYAML(test.output))

				bAsYAML, err := b.AsYAML()
				require.NoError(t, err)

				b2AsYAML, err := b2.AsYAML()
				require.NoError(t, err)

				assert.YAMLEq(t, b2AsYAML, bAsYAML)
			}
		})
	}
}

func TestStreamBuilderSetCoreYAML(t *testing.T) {
	b := service.NewStreamBuilder()
	b.SetThreads(10)
	require.NoError(t, b.SetYAML(`
input:
  kafka: {}

pipeline:
  threads: 5
  processors:
    - type: bloblang
    - type: jmespath

output:
  nats: {}
`))

	act, err := b.AsYAML()
	require.NoError(t, err)

	exp := []string{
		`input:
    label: ""
    kafka:`,
		`buffer:
    none: {}`,
		`pipeline:
    threads: 5
    processors:`,
		`
        - label: ""
          bloblang: ""`,
		`
        - label: ""
          jmespath:
            query: ""`,
		`output:
    label: ""
    nats:`,
	}

	for _, str := range exp {
		assert.Contains(t, act, str)
	}
}

func TestStreamBuilderDisabledLinting(t *testing.T) {
	lintingErrorConfig := `
input:
  kafka: {}
  meow: ignore this field

output:
  nats:
    another: linting error
`
	b := service.NewStreamBuilder()
	require.Error(t, b.SetYAML(lintingErrorConfig))

	b = service.NewStreamBuilder()
	b.DisableLinting()
	require.NoError(t, b.SetYAML(lintingErrorConfig))
}

type disabledMux struct{}

func (d disabledMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
}

func BenchmarkStreamRun(b *testing.B) {
	config := `
input:
  generate:
    count: 5
    interval: ""
    mapping: |
      root.id = uuid_v4()

pipeline:
  processors:
    - bloblang: 'root = this'

output:
  drop: {}

logger:
  level: OFF
`

	strmBuilder := service.NewStreamBuilder()
	strmBuilder.SetHTTPMux(disabledMux{})
	require.NoError(b, strmBuilder.SetYAML(config))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		strm, err := strmBuilder.Build()
		require.NoError(b, err)

		require.NoError(b, strm.Run(context.Background()))
	}
}

func BenchmarkStreamRunOutputN1(b *testing.B) {
	benchmarkStreamRunOutputNX(b, 1)
}

func BenchmarkStreamRunOutputN10(b *testing.B) {
	benchmarkStreamRunOutputNX(b, 10)
}

func BenchmarkStreamRunOutputN100(b *testing.B) {
	benchmarkStreamRunOutputNX(b, 100)
}

type noopOutput struct{}

func (n *noopOutput) Connect(ctx context.Context) error {
	return nil
}

func (n *noopOutput) Write(ctx context.Context, msg *service.Message) error {
	return nil
}

func (n *noopOutput) WriteBatch(ctx context.Context, b service.MessageBatch) error {
	return nil
}

func (n *noopOutput) Close(ctx context.Context) error {
	return nil
}

func benchmarkStreamRunOutputNX(b *testing.B, size int) {
	var outputsBuf bytes.Buffer
	for i := 0; i < size; i++ {
		outputsBuf.WriteString("      - custom: {}\n")
	}

	config := fmt.Sprintf(`
input:
  generate:
    count: 5
    interval: ""
    mapping: |
      root.id = uuid_v4()

pipeline:
  processors:
    - bloblang: 'root = this'

output:
  broker:
    outputs:
%v

logger:
  level: OFF
`, outputsBuf.String())

	env := service.NewEnvironment()
	require.NoError(b, env.RegisterOutput(
		"custom",
		service.NewConfigSpec(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (out service.Output, maxInFlight int, err error) {
			return &noopOutput{}, 1, nil
		},
	))

	strmBuilder := env.NewStreamBuilder()
	strmBuilder.SetHTTPMux(disabledMux{})
	require.NoError(b, strmBuilder.SetYAML(config))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		strm, err := strmBuilder.Build()
		require.NoError(b, err)

		require.NoError(b, strm.Run(context.Background()))
	}
}
