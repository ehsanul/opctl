package git

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"path/filepath"

	"github.com/opctl/opctl/sdks/go/data/fs"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

// New returns a data provider which sources pkgs from git repos
func New(
	basePath string,
	pullCreds *model.Creds,
) model.DataProvider {
	return &_git{
		localFSProvider: fs.New(basePath),
		basePath:        basePath,
		pullCreds:       pullCreds,
	}
}

type _git struct {
	// composed of fsProvider
	localFSProvider model.DataProvider
	basePath        string
	pullCreds       *model.Creds

	// resolveSingleFlightGroup is used to ensure resolves don't race across provider instances
	resolveSingleFlightGroup singleflight.Group
}

func (gp *_git) Label() string {
	return "git"
}

func (gp *_git) TryResolve(
	ctx context.Context,
	dataRef string,
) (model.DataHandle, error) {
	// attempt to resolve within singleFlight.Group to ensure concurrent resolves don't race
	handle, err, _ := gp.resolveSingleFlightGroup.Do(
		dataRef,
		func() (interface{}, error) {
			parsedPkgRef, err := parseRef(dataRef)
			if err != nil {
				return nil, errors.Wrap(err, "invalid git ref")
			}

			// attempt to resolve from cache
			handle, _ := gp.localFSProvider.TryResolve(ctx, dataRef)
			// ignore errors from local resolution, since we'll try to pull from a remote
			if handle != nil {
				return handle, nil
			}

			// attempt pull if cache miss
			if err := gp.pull(ctx, parsedPkgRef); err != nil {
				return nil, err
			}
			return newHandle(filepath.Join(gp.basePath, dataRef), dataRef), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return handle.(model.DataHandle), nil
}
