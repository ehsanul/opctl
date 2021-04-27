// Package containerruntime defines an interface abstracting container runtime interactions.
// A fake implementation is included to allow faking said interactions.
package containerruntime

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"io"

	"github.com/opctl/opctl/sdks/go/model"
)

// ContainerRuntime defines the interface container runtimes must implement to be supported by
//counterfeiter:generate -o fakes/containerRuntime.go . ContainerRuntime
type ContainerRuntime interface {
	DeleteContainerIfExists(
		ctx context.Context,
		containerID string,
	) error

	// RunContainer creates, starts, and waits on a container. ExitCode &/Or an error will be returned
	RunContainer(
		ctx context.Context,
		eventChannel chan model.Event,
		req *model.ContainerCall,
		// @TODO: get rid of in combination with eventPublisher
		rootCallID string,
		stdout io.WriteCloser,
		stderr io.WriteCloser,
	) (*int64, error)
}
