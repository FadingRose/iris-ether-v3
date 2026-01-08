package provider

import (
	"testing"
)

func TestLocalProvider_GetRuntimeBinaryAndProbes(t *testing.T) {
	p, err := NewLocalBenchmarkDB("test.db")
	if err != nil {
		t.Fatal(err)
	}
	data, probes, err := p.GetRuntimeBinaryAndProbes("0x0000000000000000000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("data is empty")
	}
	if len(probes) == 0 {
		t.Fatal("probes is empty")
	}
}
