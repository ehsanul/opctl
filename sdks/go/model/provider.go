package model

import (
	"context"
)

// DataProvider is the interface for something that provides data
type DataProvider interface {
	Label() string

	// TryResolve resolves a package from the source.
	//
	// expected errs:
	//  - ErrDataProviderAuthentication on authentication failure
	//  - ErrDataProviderAuthorization on authorization failure
	TryResolve(
		ctx context.Context,
		eventChannel chan Event,
		callID string,
		dataRef string,
	) (DataHandle, error)
}
