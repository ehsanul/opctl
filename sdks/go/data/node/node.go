package node

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"

	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node"
)

// New returns a data provider which sources pkgs from a node
// A node now represents a local installation of opctl, where pkgs can be
// installed into opctl's data directory.
func New(node node.Node) model.DataProvider {
	return _node{
		node: node,
	}
}

type _node struct {
	node node.Node
}

func (np _node) Label() string {
	return "opctl node"
}

func (np _node) TryResolve(
	ctx context.Context,
	eventChannel chan model.Event,
	callID string,
	dataRef string,
) (model.DataHandle, error) {

	// ensure resolvable by listing contents w/out err
	if _, err := np.node.ListDescendants(
		ctx,
		eventChannel,
		callID,
		model.ListDescendantsReq{
			PkgRef: dataRef,
		},
	); err != nil {
		return nil, err
	}

	return newHandle(np.node, dataRef), nil
}
