// Package provider provides the benchmark data from local database, file systems, or remote API
package provider

type Provider interface {
	GetRuntimeBinary(address string) ([]byte, error)
	GetRuntimeBinaryAndProbes(address string) ([]byte, []byte, error)
}
