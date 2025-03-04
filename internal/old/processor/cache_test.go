package processor

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/manager/mock"
	"github.com/benthosdev/benthos/v4/internal/message"
)

func TestCacheSet(t *testing.T) {
	mgr := mock.NewManager()
	mgr.Caches["foocache"] = map[string]mock.CacheItem{}

	conf := NewConfig()
	conf.Type = "cache"
	conf.Cache.Operator = "set"
	conf.Cache.Key = "${!json(\"key\")}"
	conf.Cache.Value = "${!json(\"value\")}"
	conf.Cache.Resource = "foocache"
	proc, err := New(conf, mgr, log.Noop(), metrics.Noop())
	if err != nil {
		t.Error(err)
		return
	}

	input := message.QuickBatch([][]byte{
		[]byte(`{"key":"1","value":"foo 1"}`),
		[]byte(`{"key":"2","value":"foo 2"}`),
		[]byte(`{"key":"1","value":"foo 3"}`),
	})

	output, res := proc.ProcessMessage(input)
	if res != nil {
		t.Fatal(res)
	}

	if len(output) != 1 {
		t.Fatalf("Wrong count of result messages: %v", len(output))
	}

	if exp, act := message.GetAllBytes(input), message.GetAllBytes(output[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result messages: %s != %s", act, exp)
	}

	actV, ok := mgr.Caches["foocache"]["1"]
	require.True(t, ok)
	assert.Equal(t, "foo 3", actV.Value)

	actV, ok = mgr.Caches["foocache"]["2"]
	require.True(t, ok)
	assert.Equal(t, "foo 2", actV.Value)
}

func TestCacheAdd(t *testing.T) {
	mgr := mock.NewManager()
	mgr.Caches["foocache"] = map[string]mock.CacheItem{}

	conf := NewConfig()
	conf.Type = "cache"
	conf.Cache.Key = "${!json(\"key\")}"
	conf.Cache.Value = "${!json(\"value\")}"
	conf.Cache.Resource = "foocache"
	conf.Cache.Operator = "add"
	proc, err := New(conf, mgr, log.Noop(), metrics.Noop())
	if err != nil {
		t.Error(err)
		return
	}

	input := message.QuickBatch([][]byte{
		[]byte(`{"key":"1","value":"foo 1"}`),
		[]byte(`{"key":"2","value":"foo 2"}`),
		[]byte(`{"key":"1","value":"foo 3"}`),
	})

	output, res := proc.ProcessMessage(input)
	if res != nil {
		t.Fatal(res)
	}

	if len(output) != 1 {
		t.Fatalf("Wrong count of result messages: %v", len(output))
	}

	if exp, act := message.GetAllBytes(input), message.GetAllBytes(output[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result messages: %s != %s", act, exp)
	}

	if exp, act := false, HasFailed(output[0].Get(0)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}
	if exp, act := false, HasFailed(output[0].Get(1)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}
	if exp, act := true, HasFailed(output[0].Get(2)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}

	actV, ok := mgr.Caches["foocache"]["1"]
	require.True(t, ok)
	assert.Equal(t, "foo 1", actV.Value)

	actV, ok = mgr.Caches["foocache"]["2"]
	require.True(t, ok)
	assert.Equal(t, "foo 2", actV.Value)
}

func TestCacheGet(t *testing.T) {
	mgr := mock.NewManager()
	mgr.Caches["foocache"] = map[string]mock.CacheItem{
		"1": {Value: "foo 1"},
		"2": {Value: "foo 2"},
	}

	conf := NewConfig()
	conf.Type = "cache"
	conf.Cache.Key = "${!json(\"key\")}"
	conf.Cache.Resource = "foocache"
	conf.Cache.Operator = "get"
	proc, err := New(conf, mgr, log.Noop(), metrics.Noop())
	if err != nil {
		t.Error(err)
		return
	}

	input := message.QuickBatch([][]byte{
		[]byte(`{"key":"1"}`),
		[]byte(`{"key":"2"}`),
		[]byte(`{"key":"3"}`),
	})
	expParts := [][]byte{
		[]byte(`foo 1`),
		[]byte(`foo 2`),
		[]byte(`{"key":"3"}`),
	}

	output, res := proc.ProcessMessage(input)
	if res != nil {
		t.Fatal(res)
	}

	if len(output) != 1 {
		t.Fatalf("Wrong count of result messages: %v", len(output))
	}

	if exp, act := expParts, message.GetAllBytes(output[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result messages: %s != %s", act, exp)
	}

	if exp, act := false, HasFailed(output[0].Get(0)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}
	if exp, act := false, HasFailed(output[0].Get(1)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}
	if exp, act := true, HasFailed(output[0].Get(2)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}
}

func TestCacheDelete(t *testing.T) {
	mgr := mock.NewManager()
	mgr.Caches["foocache"] = map[string]mock.CacheItem{
		"1": {Value: "foo 1"},
		"2": {Value: "foo 2"},
		"3": {Value: "foo 3"},
	}

	conf := NewConfig()
	conf.Type = "cache"
	conf.Cache.Key = "${!json(\"key\")}"
	conf.Cache.Resource = "foocache"
	conf.Cache.Operator = "delete"
	proc, err := New(conf, mgr, log.Noop(), metrics.Noop())
	if err != nil {
		t.Error(err)
		return
	}

	input := message.QuickBatch([][]byte{
		[]byte(`{"key":"1"}`),
		[]byte(`{"key":"3"}`),
		[]byte(`{"key":"4"}`),
	})

	output, res := proc.ProcessMessage(input)
	if res != nil {
		t.Fatal(res)
	}

	if len(output) != 1 {
		t.Fatalf("Wrong count of result messages: %v", len(output))
	}

	if exp, act := message.GetAllBytes(input), message.GetAllBytes(output[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result messages: %s != %s", act, exp)
	}

	if exp, act := false, HasFailed(output[0].Get(0)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}
	if exp, act := false, HasFailed(output[0].Get(1)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}
	if exp, act := false, HasFailed(output[0].Get(2)); exp != act {
		t.Errorf("Wrong fail flag: %v != %v", act, exp)
	}

	_, ok := mgr.Caches["foocache"]["1"]
	require.False(t, ok)

	actV, ok := mgr.Caches["foocache"]["2"]
	require.True(t, ok)
	assert.Equal(t, "foo 2", actV.Value)

	_, ok = mgr.Caches["foocache"]["3"]
	require.False(t, ok)
}
