package input

import (
	"context"
	"fmt"
	"time"

	"github.com/benthosdev/benthos/v4/internal/component/input"
	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/docs"
	"github.com/benthosdev/benthos/v4/internal/interop"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/message"
)

func init() {
	Constructors[TypeResource] = TypeSpec{
		constructor: fromSimpleConstructor(NewResource),
		Summary: `
Resource is an input type that runs a resource input by its name.`,
		Description: `
This input allows you to reference the same configured input resource in multiple places, and can also tidy up large nested configs. For
example, the config:

` + "```yaml" + `
input:
  broker:
    inputs:
      - kafka:
          addresses: [ TODO ]
          topics: [ foo ]
          consumer_group: foogroup
      - gcp_pubsub:
          project: bar
          subscription: baz
` + "```" + `

Could also be expressed as:

` + "```yaml" + `
input:
  broker:
    inputs:
      - resource: foo
      - resource: bar

input_resources:
  - label: foo
    kafka:
      addresses: [ TODO ]
      topics: [ foo ]
      consumer_group: foogroup

  - label: bar
    gcp_pubsub:
      project: bar
      subscription: baz
 ` + "```" + `

You can find out more about resources [in this document.](/docs/configuration/resources)`,
		Categories: []string{
			"Utility",
		},
		Config: docs.FieldString("", "").HasDefault(""),
	}
}

//------------------------------------------------------------------------------

// Resource is an input that wraps an input resource.
type Resource struct {
	mgr  interop.Manager
	name string
	log  log.Modular
}

// NewResource returns a resource input.
func NewResource(
	conf Config, mgr interop.Manager, log log.Modular, stats metrics.Type,
) (input.Streamed, error) {
	if !mgr.ProbeInput(conf.Resource) {
		return nil, fmt.Errorf("input resource '%v' was not found", conf.Resource)
	}
	return &Resource{
		mgr:  mgr,
		name: conf.Resource,
		log:  log,
	}, nil
}

//------------------------------------------------------------------------------

// TransactionChan returns a transactions channel for consuming messages from
// this input type.
func (r *Resource) TransactionChan() (tChan <-chan message.Transaction) {
	if err := r.mgr.AccessInput(context.Background(), r.name, func(i input.Streamed) {
		tChan = i.TransactionChan()
	}); err != nil {
		r.log.Errorf("Failed to obtain input resource '%v': %v", r.name, err)
	}
	return
}

// Connected returns a boolean indicating whether this input is currently
// connected to its target.
func (r *Resource) Connected() (isConnected bool) {
	if err := r.mgr.AccessInput(context.Background(), r.name, func(i input.Streamed) {
		isConnected = i.Connected()
	}); err != nil {
		r.log.Errorf("Failed to obtain input resource '%v': %v", r.name, err)
	}
	return
}

// CloseAsync shuts down the processor and stops processing requests.
func (r *Resource) CloseAsync() {
}

// WaitForClose blocks until the processor has closed down.
func (r *Resource) WaitForClose(timeout time.Duration) error {
	return nil
}
