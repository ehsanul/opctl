package dataresolver

import (
	"context"
	"os"
	"path/filepath"

	"github.com/opctl/opctl/cli/internal/cliparamsatisfier"
	"github.com/opctl/opctl/sdks/go/data"
	"github.com/opctl/opctl/sdks/go/data/fs"
	dataNode "github.com/opctl/opctl/sdks/go/data/node"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node"
)

// DataResolver resolves packages
type DataResolver interface {
	Resolve(
		ctx context.Context,
		eventChannel chan model.Event,
		callID string,
		dataRef string,
	) (model.DataHandle, error)
}

func New(
	cliParamSatisfier cliparamsatisfier.CLIParamSatisfier,
	node node.Node,
) DataResolver {
	return _dataResolver{
		cliParamSatisfier: cliParamSatisfier,
		node:              node,
	}
}

type _dataResolver struct {
	cliParamSatisfier cliparamsatisfier.CLIParamSatisfier
	node              node.Node
}

func (dtr _dataResolver) Resolve(
	ctx context.Context,
	eventChannel chan model.Event,
	callID string,
	dataRef string,
) (model.DataHandle, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	fsProvider := fs.New(
		filepath.Join(cwd, ".opspec"),
		cwd,
	)

	opDirHandle, err := data.Resolve(
		ctx,
		eventChannel,
		callID,
		dataRef,
		fsProvider,
		dataNode.New(
			dtr.node,
		),
	)

	return opDirHandle, err
}
