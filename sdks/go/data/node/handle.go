package node

import (
	"context"

	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node"
)

func newHandle(
	node node.Node,
	dataRef string,
) model.DataHandle {
	return handle{
		node:    node,
		dataRef: dataRef,
	}
}

// handle allows interacting w/ data sourced from an opspec node
type handle struct {
	node    node.Node
	dataRef string
}

func (nh handle) GetContent(
	ctx context.Context,
	eventChannel chan model.Event,
	callID string,
	contentPath string,
) (
	model.ReadSeekCloser,
	error,
) {
	return nh.node.GetData(
		ctx,
		eventChannel,
		callID,
		model.GetDataReq{
			ContentPath: contentPath,
			PkgRef:      nh.dataRef,
		},
	)
}

func (nh handle) ListDescendants(
	ctx context.Context,
	eventChannel chan model.Event,
	callID string,
) (
	[]*model.DirEntry,
	error,
) {
	return nh.node.ListDescendants(
		ctx,
		eventChannel,
		callID,
		model.ListDescendantsReq{
			PkgRef: nh.dataRef,
		},
	)
}

func (handle) Path() *string {
	return nil
}

func (nh handle) Ref() string {
	return nh.dataRef
}
