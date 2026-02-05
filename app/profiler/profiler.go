// Package profiler provides functionality to integrate with Pyroscope for application profiling.
package profiler

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/grafana/pyroscope-go"
)

// PyroscopeProfiler represents a profiler that sends profiling data to a Pyroscope server.
type PyroscopeProfiler struct {
	endpoint string
	tags     map[string]string
	profiler *pyroscope.Profiler
}

// NewPyroscopeProfiler creates a new PyroscopeProfiler instance.
func NewPyroscopeProfiler(endpoint string, tags map[string]string) (*PyroscopeProfiler, error) {
	if endpoint == "" {
		return nil, errors.New("profiler endpoint is required")
	}

	return &PyroscopeProfiler{
		endpoint: endpoint,
		tags:     tags,
		profiler: nil,
	}, nil
}

// Endpoint returns the Pyroscope server endpoint.
func (p *PyroscopeProfiler) Endpoint() string {
	return p.endpoint
}

// Start initializes and starts the Pyroscope profiler.
func (p *PyroscopeProfiler) Start() error {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "simtezilo",
		ServerAddress:   p.endpoint,
		Logger:          pyroscope.StandardLogger,
		Tags:            p.tags,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	if err != nil {
		return fmt.Errorf("pyroscope.Start(): %w", err)
	}

	p.profiler = profiler

	return nil
}

// Shutdown stops the Pyroscope profiler and cleans up resources.
func (p *PyroscopeProfiler) Shutdown() error {
	if p.profiler == nil {
		return nil
	}

	err := p.profiler.Stop()
	if err != nil {
		return fmt.Errorf("pyroscope.Stop(): %w", err)
	}

	p.profiler = nil

	return nil
}
