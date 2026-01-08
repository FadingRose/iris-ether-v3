package provider

import (
	"fadingrose/iris-ether-v3/db"
	"fmt"
)

type LocalProvider struct {
	instance *db.Database
	builder  *db.QueryBuilder
}

func NewLocalBenchmarkDB(path string) (*LocalProvider, error) {
	instance, err := db.OpenOrCreateDBFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open local benchmark database: %w", err)
	}
	schema, err := instance.GetSchema("runtime_binary")
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime_binary schema: %w", err)
	}
	return &LocalProvider{
		instance: instance,
		builder:  schema.NewQuery().Clean(),
	}, nil
}

func (p *LocalProvider) GetRuntimeBinary(address string) ([]byte, error) {
	p.builder.Where("address = ?").WithArgs(address).Select("address", "runtime_bin")
	defer p.builder.Clean()
	query, args, err := p.builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}
	rows, err := p.instance.Query(query, args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	if rows.Next() {
		var address string
		var data []byte
		err := rows.Scan(&address, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("address %s not found", address)
}

func (p *LocalProvider) GetRuntimeBinaryAndProbes(address string) ([]byte, []byte, error) {
	p.builder.Where("address = ?").WithArgs(address)
	defer p.builder.Clean()
	query, args, err := p.builder.Build()
	if err != nil {
		return nil, nil, fmt.Errorf("[GetRuntimeBinaryAndProbes]failed to build query: %w", err)
	}
	rows, err := p.instance.Query(query, args[0])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query: %w", err)
	}
	if rows.Next() {
		var id int
		var bytecode []byte
		var probes []byte
		var address string
		var createdAt string
		err := rows.Scan(&id, &address, &bytecode, &probes, &createdAt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}
		return bytecode, probes, nil
	}
	return nil, nil, fmt.Errorf("address %s not found", address)
}
