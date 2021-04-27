package core

import (
	"context"
	"fmt"
	"time"

	"github.com/opctl/opctl/sdks/go/model"
	callpkg "github.com/opctl/opctl/sdks/go/opspec/interpreter/call"
)

//counterfeiter:generate -o internal/fakes/caller.go . caller
type caller interface {
	// Call executes a call
	Call(
		ctx context.Context,
		eventChannel chan model.Event,
		id string,
		scope map[string]*model.Value,
		callSpec *model.CallSpec,
		opPath string,
		parentCallID *string,
		rootCallID string,
	) (
		map[string]*model.Value,
		error,
	)
}

func newCaller(
	containerCaller containerCaller,
	dataDirPath string,
) caller {
	instance := &_caller{
		containerCaller: containerCaller,
		dataDirPath:     dataDirPath,
	}
	instance.opCaller = newOpCaller(instance, dataDirPath)
	instance.parallelCaller = newParallelCaller(instance)
	instance.parallelLoopCaller = newParallelLoopCaller(instance)
	instance.serialCaller = newSerialCaller(instance)
	instance.serialLoopCaller = newSerialLoopCaller(instance)

	return instance
}

type _caller struct {
	containerCaller    containerCaller
	dataDirPath        string
	opCaller           opCaller
	parallelCaller     parallelCaller
	parallelLoopCaller parallelLoopCaller
	serialCaller       serialCaller
	serialLoopCaller   serialLoopCaller
}

func (clr _caller) Call(
	ctx context.Context,
	eventChannel chan model.Event,
	id string,
	scope map[string]*model.Value,
	callSpec *model.CallSpec,
	opPath string,
	parentCallID *string,
	rootCallID string,
) (
	map[string]*model.Value,
	error,
) {
	callCtx, cancelCall := context.WithCancel(ctx)
	defer cancelCall()
	var err error
	var outputs map[string]*model.Value
	var call *model.Call
	callStartTime := time.Now().UTC()

	if callCtx.Err() != nil {
		// if context done NOOP
		return nil, nil
	}

	// emit a call ended event after this call is complete
	defer func() {
		// defer must be defined before conditional return statements so it always runs

		if call == nil {
			call = &model.Call{
				ID:       id,
				RootID:   rootCallID,
				ParentID: parentCallID,
			}
		}

		event := model.Event{
			CallEnded: &model.CallEnded{
				Call:    *call,
				Outputs: outputs,
				Ref:     opPath,
			},
			Timestamp: time.Now().UTC(),
		}

		if ctx.Err() != nil {
			// this call or parent call killed/cancelled
			event.CallEnded.Outcome = model.OpOutcomeKilled
			event.CallEnded.Error = &model.CallEndedError{
				Message: ctx.Err().Error(),
			}
		} else if err != nil {
			event.CallEnded.Outcome = model.OpOutcomeFailed
			event.CallEnded.Error = &model.CallEndedError{
				Message: err.Error(),
			}
		} else {
			event.CallEnded.Outcome = model.OpOutcomeSucceeded
		}

		eventChannel <- event
	}()

	if callSpec == nil {
		// NOOP
		return outputs, err
	}

	// scratchDirPath := filepath.Join(clr.dataDirPath, "dcg", id)
	// defer func() {
	// 	os.RemoveAll(scratchDirPath)
	// }()

	call, err = callpkg.Interpret(
		ctx,
		eventChannel,
		scope,
		callSpec,
		id,
		opPath,
		parentCallID,
		rootCallID,
		clr.dataDirPath,
	)
	if err != nil {
		return nil, err
	}

	if call.If != nil && !*call.If {
		return outputs, err
	}

	// Ensure this is emitted just after the deferred operation to emit the end
	// event is set up, so we always have a matching start and end event
	eventChannel <- model.Event{
		Timestamp: callStartTime,
		CallStarted: &model.CallStarted{
			Call: *call,
			Ref:  opPath,
		},
	}

	switch {
	case callSpec.Container != nil:
		outputs, err = clr.containerCaller.Call(
			callCtx,
			eventChannel,
			call.Container,
			scope,
			callSpec.Container,
			rootCallID,
		)
	case callSpec.Op != nil:
		outputs, err = clr.opCaller.Call(
			callCtx,
			eventChannel,
			call.Op,
			scope,
			parentCallID,
			rootCallID,
			callSpec.Op,
		)
	case callSpec.Parallel != nil:
		outputs, err = clr.parallelCaller.Call(
			callCtx,
			eventChannel,
			id,
			scope,
			rootCallID,
			opPath,
			*callSpec.Parallel,
		)
	case callSpec.ParallelLoop != nil:
		outputs, err = clr.parallelLoopCaller.Call(
			callCtx,
			eventChannel,
			id,
			scope,
			*callSpec.ParallelLoop,
			opPath,
			parentCallID,
			rootCallID,
		)
	case callSpec.Serial != nil:
		outputs, err = clr.serialCaller.Call(
			callCtx,
			eventChannel,
			id,
			scope,
			rootCallID,
			opPath,
			*callSpec.Serial,
		)
	case callSpec.SerialLoop != nil:
		outputs, err = clr.serialLoopCaller.Call(
			callCtx,
			eventChannel,
			id,
			scope,
			*callSpec.SerialLoop,
			opPath,
			parentCallID,
			rootCallID,
		)
	default:
		err = fmt.Errorf("invalid call graph '%+v'", callSpec)
	}

	return outputs, err
}
