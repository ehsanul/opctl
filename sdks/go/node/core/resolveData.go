package core

import (
	"context"

	"github.com/opctl/opctl/sdks/go/data"
	"github.com/opctl/opctl/sdks/go/data/fs"
	"github.com/opctl/opctl/sdks/go/data/git"
	"github.com/opctl/opctl/sdks/go/model"
)

// Resolve attempts to resolve data via local filesystem or git
// nil pullCreds will be ignored
//
// expected errs:
//  - ErrDataProviderAuthentication on authentication failure
//  - ErrDataProviderAuthorization on authorization failure
func (cr core) ResolveData(
	ctx context.Context,
	eventChannel chan model.Event,
	callID string,
	dataRef string,
) (
	model.DataHandle,
	error,
) {
	return data.Resolve(
		ctx,
		eventChannel,
		callID,
		dataRef,
		fs.New(),
		git.New(cr.dataCachePath),
	)
}
