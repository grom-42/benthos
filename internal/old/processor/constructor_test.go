package processor_test

import (
	"testing"

	yaml "gopkg.in/yaml.v3"

	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/manager/mock"
	"github.com/benthosdev/benthos/v4/internal/old/processor"

	_ "github.com/benthosdev/benthos/v4/public/components/all"
)

func TestConstructorBadType(t *testing.T) {
	conf := processor.NewConfig()
	conf.Type = "not_exist"

	if _, err := processor.New(conf, mock.NewManager(), log.Noop(), metrics.Noop()); err == nil {
		t.Error("Expected error, received nil for invalid type")
	}
}

func TestConstructorConfigYAMLInference(t *testing.T) {
	conf := []processor.Config{}

	if err := yaml.Unmarshal([]byte(`[
		{
			"bloblang": "root = this",
			"jmespath": {
				"query": "foo"
			}
		}
	]`), &conf); err == nil {
		t.Error("Expected error from multi candidates")
	}

	if err := yaml.Unmarshal([]byte(`[
		{
			"bloblang": "root = this"
		}
	]`), &conf); err != nil {
		t.Error(err)
	}

	if exp, act := 1, len(conf); exp != act {
		t.Errorf("Wrong number of config parts: %v != %v", act, exp)
		return
	}
	if exp, act := processor.TypeBloblang, conf[0].Type; exp != act {
		t.Errorf("Wrong inferred type: %v != %v", act, exp)
	}
}

func TestConstructorConfigDefaultsYAML(t *testing.T) {
	conf := []processor.Config{}

	if err := yaml.Unmarshal([]byte(`[
		{
			"type": "bounds_check",
			"bounds_check": {
				"max_part_size": 50
			}
		}
	]`), &conf); err != nil {
		t.Error(err)
	}

	if exp, act := 1, len(conf); exp != act {
		t.Errorf("Wrong number of config parts: %v != %v", act, exp)
		return
	}
	if exp, act := 100, conf[0].BoundsCheck.MaxParts; exp != act {
		t.Errorf("Wrong default parts: %v != %v", act, exp)
	}
	if exp, act := 50, conf[0].BoundsCheck.MaxPartSize; exp != act {
		t.Errorf("Wrong overridden part size: %v != %v", act, exp)
	}
}
