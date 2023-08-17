package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

var MockNodeInfoFetcher = &MockFetcher{fixture: "fixtures/sinfo_out.json"}

func TestParseNodeMetrics(t *testing.T) {
	fixture, err := MockNodeInfoFetcher.Fetch()
	if err != nil {
		t.Fatalf("Failed to read file with %q", err)
	}
	metrics, err := parseNodeMetrics(fixture)
	if err != nil {
		t.Fatalf("Failed to parse metrics with %s", err)
	}
	if len(metrics) == 0 {
		t.Fatal("No metrics recieved")
	}
	t.Logf("Node metrics collected %d", len(metrics))
}

func TestPartitionMetric(t *testing.T) {
	assert := assert.New(t)
	fixture, err := MockNodeInfoFetcher.Fetch()
	assert.NoError(err)
	nodeMetrics, err := parseNodeMetrics(fixture)
	assert.Nil(err)
	metrics := fetchNodePartitionMetrics(nodeMetrics)
	assert.Equal(1, len(metrics))
	_, contains := metrics["hw"]
	assert.True(contains)
	assert.Equal(4., metrics["hw"].AllocCpus)
	assert.Equal(256., metrics["hw"].Cpus)
	assert.Equal(114688., metrics["hw"].AllocMemory)
	assert.Equal(1.823573e+06, metrics["hw"].FreeMemory)
	assert.Equal(2e+06, metrics["hw"].RealMemory)
	assert.Equal(252., metrics["hw"].IdleCpus)
}

func TestNodeSummaryCpuMetric(t *testing.T) {
	assert := assert.New(t)
	fixture, err := MockNodeInfoFetcher.Fetch()
	assert.NoError(err)
	nodeMetrics, err := parseNodeMetrics(fixture)
	assert.Nil(err)
	metrics := fetchNodeTotalCpuMetrics(nodeMetrics)
	assert.Equal(4, len(metrics.PerState))
	for _, cpu := range metrics.PerState {
		assert.Equal(64., cpu)
	}
}

func TestNodeSummaryMemoryMetrics(t *testing.T) {
	assert := assert.New(t)
	fixture, err := MockNodeInfoFetcher.Fetch()
	assert.NoError(err)
	nodeMetrics, err := parseNodeMetrics(fixture)
	assert.Nil(err)
	metrics := fetchNodeTotalMemMetrics(nodeMetrics)
	assert.Equal(114688., metrics.AllocMemory)
	assert.Equal(1.823573e+06, metrics.FreeMemory)
	assert.Equal(2e+06, metrics.RealMemory)
}

func TestNodeCollector(t *testing.T) {
	assert := assert.New(t)
	config, err := NewConfig()
	assert.Nil(err)
	nc := NewNodeCollecter(config)
	// cache miss, use our mock fetcher
	nc.fetcher = MockNodeInfoFetcher
	metricChan := make(chan prometheus.Metric)
	go func() {
		nc.Collect(metricChan)
		close(metricChan)
	}()
	metrics := make([]prometheus.Metric, 0)
	for m, ok := <-metricChan; ok; m, ok = <-metricChan {
		metrics = append(metrics, m)
		t.Logf("Received metric %s", m.Desc().String())
	}
	assert.Positive(len(metrics))
}

func TestNodeDescribe(t *testing.T) {
	assert := assert.New(t)
	ch := make(chan *prometheus.Desc)
	config, err := NewConfig()
	assert.Nil(err)
	jc := NewNodeCollecter(config)
	jc.fetcher = MockJobInfoFetcher
	go func() {
		jc.Describe(ch)
		close(ch)
	}()
	descs := make([]*prometheus.Desc, 0)
	for desc, ok := <-ch; ok; desc, ok = <-ch {
		descs = append(descs, desc)
	}
	assert.Positive(len(descs))
}