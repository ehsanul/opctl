package node

import (
	"context"

	"github.com/opctl/opctl/sdks/go/model"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -o fakes/core.go . Node

// Node is the main structure to run and interact with ops
type Node interface {
	// AddAuth records authentication within the core
	AddAuth(
		ctx context.Context,
		req model.AddAuthReq,
	) error

	// StartOp starts an op and returns the root call ID
	StartOp(
		ctx context.Context,
		eventChannel chan model.Event,
		req model.StartOpReq,
	) (
		outputs map[string]*model.Value,
		err error,
	)

	// GetData gets data
	//
	// expected errs:
	//  - ErrDataProviderAuthentication on authentication failure
	//  - ErrDataProviderAuthorization on authorization failure
	//  - ErrDataRefResolution on resolution failure
	GetData(
		ctx context.Context,
		eventChannel chan model.Event,
		callID string,
		req model.GetDataReq,
	) (
		model.ReadSeekCloser,
		error,
	)

	// ListDescendants lists file system entries
	//
	// expected errs:
	//  - ErrDataProviderAuthentication on authentication failure
	//  - ErrDataProviderAuthorization on authorization failure
	//  - ErrDataRefResolution on resolution failure
	ListDescendants(
		ctx context.Context,
		eventChannel chan model.Event,
		callID string,
		req model.ListDescendantsReq,
	) (
		[]*model.DirEntry,
		error,
	)
}
