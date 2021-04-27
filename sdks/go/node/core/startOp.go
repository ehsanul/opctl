package core

import (
	"context"

	"github.com/opctl/opctl/sdks/go/data"
	"github.com/opctl/opctl/sdks/go/data/fs"
	"github.com/opctl/opctl/sdks/go/data/git"
	"github.com/opctl/opctl/sdks/go/internal/uniquestring"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/opspec/opfile"
)

func (this core) StartOp(
	ctx context.Context,
	eventChannel chan model.Event,
	req model.StartOpReq,
) (map[string]*model.Value, error) {
	callID, err := uniquestring.Construct()
	if err != nil {
		// end run immediately on any error
		return nil, err
	}

	opHandle, err := data.Resolve(
		ctx,
		eventChannel,
		callID,
		req.Op.Ref,
		fs.New(),
		git.New(this.dataCachePath),
	)
	if err != nil {
		return nil, err
	}

	// construct opCallSpec
	opCallSpec := &model.OpCallSpec{
		Ref:     opHandle.Ref(),
		Inputs:  map[string]interface{}{},
		Outputs: map[string]string{},
	}

	for name := range req.Args {
		// implicitly bind
		opCallSpec.Inputs[name] = ""
	}

	opFile, err := opfile.Get(
		ctx,
		*opHandle.Path(),
	)
	if err != nil {
		return nil, err
	}
	for name := range opFile.Outputs {
		// implicitly bind
		opCallSpec.Outputs[name] = ""
	}

	return this.caller.Call(
		ctx,
		eventChannel,
		callID,
		req.Args,
		&model.CallSpec{
			Op: opCallSpec,
		},
		*opHandle.Path(),
		nil,
		callID,
	)
}
