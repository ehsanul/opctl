package op

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/opctl/opctl/sdks/go/data"
	"github.com/opctl/opctl/sdks/go/data/fs"
	"github.com/opctl/opctl/sdks/go/data/git"
	"github.com/opctl/opctl/sdks/go/internal/uniquestring"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/opspec/interpreter/call/op/inputs"
	"github.com/opctl/opctl/sdks/go/opspec/interpreter/dir"
	"github.com/opctl/opctl/sdks/go/opspec/interpreter/reference"
	"github.com/opctl/opctl/sdks/go/opspec/opfile"
	"github.com/pkg/errors"
)

// Interpret interprets an OpCallSpec into a OpCall
func Interpret(
	ctx context.Context,
	scope map[string]*model.Value,
	opCallSpec *model.OpCallSpec,
	opID string,
	parentOpPath string,
	dataDirPath string,
) (*model.OpCall, error) {
	scratchDirPath := filepath.Join(dataDirPath, "dcg", opID)

	var opPath string
	if reference.ReferenceRegexp.MatchString(opCallSpec.Ref) {
		// attempt to process as a variable reference since its variable reference like.
		dirValue, err := dir.Interpret(
			scope,
			opCallSpec.Ref,
			scratchDirPath,
			false,
		)
		if err != nil {
			return nil, errors.Wrap(err, "error encountered interpreting image src")
		}
		opPath = *dirValue.Dir
	} else {
		opHandle, err := data.Resolve(
			ctx,
			opCallSpec.Ref,
			fs.New(parentOpPath, filepath.Dir(parentOpPath)),
			git.New(filepath.Join(dataDirPath, "ops")),
		)
		if err != nil {
			return nil, err
		}
		opPath = *opHandle.Path()
	}

	opFile, err := opfile.Get(
		ctx,
		opPath,
	)
	if err != nil {
		return nil, err
	}

	childCallID, err := uniquestring.Construct()
	if err != nil {
		return nil, err
	}

	opCall := &model.OpCall{
		BaseCall: model.BaseCall{
			OpPath: opPath,
		},
		ChildCallID:       childCallID,
		ChildCallCallSpec: opFile.Run,
		OpID:              opID,
	}

	opCall.Inputs, err = inputs.Interpret(
		opCallSpec.Inputs,
		opFile.Inputs,
		opPath,
		scope,
		scratchDirPath,
	)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to interpret call to %v", opCallSpec.Ref))
	}

	return opCall, nil
}
