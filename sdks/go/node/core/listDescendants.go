package core

import (
	"context"

	"github.com/opctl/opctl/sdks/go/model"
)

func (c core) ListDescendants(
	ctx context.Context,
	eventChannel chan model.Event,
	callID string,
	req model.ListDescendantsReq,
) (
	[]*model.DirEntry,
	error,
) {
	if req.PkgRef == "" {
		return []*model.DirEntry{}, nil
	}

	dataHandle, err := c.ResolveData(ctx, eventChannel, callID, req.PkgRef)
	if err != nil {
		return nil, err
	}

	return dataHandle.ListDescendants(ctx, eventChannel, callID)
}
