package processor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/Jeffail/grok"

	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/component/processor"
	"github.com/benthosdev/benthos/v4/internal/docs"
	"github.com/benthosdev/benthos/v4/internal/filepath"
	"github.com/benthosdev/benthos/v4/internal/interop"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/message"
)

//------------------------------------------------------------------------------

func init() {
	Constructors[TypeGrok] = TypeSpec{
		constructor: func(conf Config, mgr interop.Manager, log log.Modular, stats metrics.Type) (processor.V1, error) {
			p, err := newGrok(conf.Grok, mgr)
			if err != nil {
				return nil, err
			}
			return processor.NewV2ToV1Processor("grok", p, mgr.Metrics()), nil
		},
		Categories: []string{
			"Parsing",
		},
		Summary: `
Parses messages into a structured format by attempting to apply a list of Grok expressions, the first expression to result in at least one value replaces the original message with a JSON object containing the values.`,
		Description: `
Type hints within patterns are respected, therefore with the pattern ` + "`%{WORD:first},%{INT:second:int}`" + ` and a payload of ` + "`foo,1`" + ` the resulting payload would be ` + "`{\"first\":\"foo\",\"second\":1}`" + `.

### Performance

This processor currently uses the [Go RE2](https://golang.org/s/re2syntax) regular expression engine, which is guaranteed to run in time linear to the size of the input. However, this property often makes it less performant than PCRE based implementations of grok. For more information see [https://swtch.com/~rsc/regexp/regexp1.html](https://swtch.com/~rsc/regexp/regexp1.html).`,
		Config: docs.FieldComponent().WithChildren(
			docs.FieldString("expressions", "One or more Grok expressions to attempt against incoming messages. The first expression to match at least one value will be used to form a result.").Array(),
			docs.FieldString("pattern_definitions", "A map of pattern definitions that can be referenced within `patterns`.").Map(),
			docs.FieldString("pattern_paths", "A list of paths to load Grok patterns from. This field supports wildcards, including super globs (double star).").Array(),
			docs.FieldBool("named_captures_only", "Whether to only capture values from named patterns.").Advanced(),
			docs.FieldBool("use_default_patterns", "Whether to use a [default set of patterns](#default-patterns).").Advanced(),
			docs.FieldBool("remove_empty_values", "Whether to remove values that are empty from the resulting structure.").Advanced(),
		),
		Examples: []docs.AnnotatedExample{
			{
				Title: "VPC Flow Logs",
				Summary: `
Grok can be used to parse unstructured logs such as VPC flow logs that look like this:

` + "```text" + `
2 123456789010 eni-1235b8ca123456789 172.31.16.139 172.31.16.21 20641 22 6 20 4249 1418530010 1418530070 ACCEPT OK
` + "```" + `

Into structured objects that look like this:

` + "```json" + `
{"accountid":"123456789010","action":"ACCEPT","bytes":4249,"dstaddr":"172.31.16.21","dstport":22,"end":1418530070,"interfaceid":"eni-1235b8ca123456789","logstatus":"OK","packets":20,"protocol":6,"srcaddr":"172.31.16.139","srcport":20641,"start":1418530010,"version":2}
` + "```" + `

With the following config:`,
				Config: `
pipeline:
  processors:
    - grok:
        expressions:
          - '%{VPCFLOWLOG}'
        pattern_definitions:
          VPCFLOWLOG: '%{NUMBER:version:int} %{NUMBER:accountid} %{NOTSPACE:interfaceid} %{NOTSPACE:srcaddr} %{NOTSPACE:dstaddr} %{NOTSPACE:srcport:int} %{NOTSPACE:dstport:int} %{NOTSPACE:protocol:int} %{NOTSPACE:packets:int} %{NOTSPACE:bytes:int} %{NUMBER:start:int} %{NUMBER:end:int} %{NOTSPACE:action} %{NOTSPACE:logstatus}'
`,
			},
		},
		Footnotes: `
## Default Patterns

A summary of the default patterns on offer can be [found here](https://github.com/Jeffail/grok/blob/master/patterns.go#L5).`,
	}
}

//------------------------------------------------------------------------------

// GrokConfig contains configuration fields for the Grok processor.
type GrokConfig struct {
	Expressions        []string          `json:"expressions" yaml:"expressions"`
	RemoveEmpty        bool              `json:"remove_empty_values" yaml:"remove_empty_values"`
	NamedOnly          bool              `json:"named_captures_only" yaml:"named_captures_only"`
	UseDefaults        bool              `json:"use_default_patterns" yaml:"use_default_patterns"`
	PatternPaths       []string          `json:"pattern_paths" yaml:"pattern_paths"`
	PatternDefinitions map[string]string `json:"pattern_definitions" yaml:"pattern_definitions"`
}

// NewGrokConfig returns a GrokConfig with default values.
func NewGrokConfig() GrokConfig {
	return GrokConfig{
		Expressions:        []string{},
		RemoveEmpty:        true,
		NamedOnly:          true,
		UseDefaults:        true,
		PatternPaths:       []string{},
		PatternDefinitions: make(map[string]string),
	}
}

//------------------------------------------------------------------------------

type grokProc struct {
	gparsers []*grok.CompiledGrok
	log      log.Modular
}

func newGrok(conf GrokConfig, mgr interop.Manager) (processor.V2, error) {
	grokConf := grok.Config{
		RemoveEmptyValues:   conf.RemoveEmpty,
		NamedCapturesOnly:   conf.NamedOnly,
		SkipDefaultPatterns: !conf.UseDefaults,
		Patterns:            conf.PatternDefinitions,
	}

	for _, path := range conf.PatternPaths {
		if err := addGrokPatternsFromPath(path, grokConf.Patterns); err != nil {
			return nil, fmt.Errorf("failed to parse patterns from path '%v': %v", path, err)
		}
	}

	gcompiler, err := grok.New(grokConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create grok compiler: %v", err)
	}

	var compiled []*grok.CompiledGrok
	for _, pattern := range conf.Expressions {
		var gcompiled *grok.CompiledGrok
		if gcompiled, err = gcompiler.Compile(pattern); err != nil {
			return nil, fmt.Errorf("failed to compile Grok pattern '%v': %v", pattern, err)
		}
		compiled = append(compiled, gcompiled)
	}

	g := &grokProc{
		gparsers: compiled,
		log:      mgr.Logger(),
	}
	return g, nil
}

//------------------------------------------------------------------------------

func addGrokPatternsFromPath(path string, patterns map[string]string) error {
	if s, err := os.Stat(path); err != nil {
		return err
	} else if s.IsDir() {
		path += "/*"
	}

	files, err := filepath.Globs([]string{path})
	if err != nil {
		return err
	}

	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			l := scanner.Text()
			if len(l) > 0 && l[0] != '#' {
				names := strings.SplitN(l, " ", 2)
				patterns[names[0]] = names[1]
			}
		}

		file.Close()
	}

	return nil
}

func (g *grokProc) Process(ctx context.Context, msg *message.Part) ([]*message.Part, error) {
	body := msg.Get()

	var values map[string]interface{}
	for _, compiler := range g.gparsers {
		var err error
		if values, err = compiler.ParseTyped(body); err != nil {
			g.log.Debugf("Failed to parse body: %v\n", err)
			continue
		}
		if len(values) > 0 {
			break
		}
	}
	if len(values) == 0 {
		g.log.Debugf("No matches found for payload: %s\n", body)
		return nil, errors.New("no pattern matches found")
	}

	gObj := gabs.New()
	for k, v := range values {
		gObj.SetP(v, k)
	}

	newMsg := msg.Copy()
	newMsg.SetJSON(gObj.Data())

	return []*message.Part{newMsg}, nil
}

func (g *grokProc) Close(context.Context) error {
	return nil
}
