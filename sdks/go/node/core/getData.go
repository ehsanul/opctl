package core

import (
	"context"
	"fmt"

	"github.com/opctl/opctl/sdks/go/model"
)

func (c core) GetData(
	ctx context.Context,
	eventChannel chan model.Event,
	callID string,
	req model.GetDataReq,
) (
	model.ReadSeekCloser,
	error,
) {
	if req.PkgRef == "" || req.ContentPath == "" {
		return nil, fmt.Errorf("invalid ref: %s%s", req.PkgRef, req.ContentPath)
	}

	dataHandle, err := c.ResolveData(ctx, eventChannel, callID, req.PkgRef)
	if err != nil {
		return nil, err
	}

	return dataHandle.GetContent(ctx, eventChannel, callID, req.ContentPath)
}
