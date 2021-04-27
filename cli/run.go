package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/opctl/opctl/cli/internal/clioutput"
	"github.com/opctl/opctl/cli/internal/cliparamsatisfier"
	"github.com/opctl/opctl/cli/internal/dataresolver"
	"github.com/opctl/opctl/cli/internal/opgraph"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/core"
	"github.com/opctl/opctl/sdks/go/opspec/opfile"
	"github.com/pkg/errors"
)

// RunOpts are options to run a given op through the CLI
type RunOpts struct {
	ArgFile string
	Args    []string
}

type runResults struct {
	outputs map[string]*model.Value
	err     error
}

func run(
	ctx context.Context,
	cliOutput clioutput.CliOutput,
	cliParamSatisfier cliparamsatisfier.CLIParamSatisfier,
	eventChannel chan model.Event,
	node core.Core,
	opFormatter clioutput.OpFormatter,
	opRef string,
	opts *RunOpts,
	displayLiveGraph bool,
) (map[string]*model.Value, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	aSigIntWasReceivedAlready := false
	sigIntChannel := make(chan os.Signal, 1)
	defer close(sigIntChannel)
	signal.Notify(
		sigIntChannel,
		syscall.SIGINT,
	)

	sigTermChannel := make(chan os.Signal, 1)
	defer close(sigTermChannel)
	signal.Notify(
		sigTermChannel,
		syscall.SIGTERM,
	)

	sigInfoChannel := make(chan os.Signal, 1)
	defer close(sigInfoChannel)
	signal.Notify(
		sigInfoChannel,
		syscall.Signal(0x1d), // portable version of syscall.SIGINFO
	)

	// "request animation frame" like loop to force refresh of display loading spinners
	animationFrame := make(chan bool)
	if displayLiveGraph {
		go func() {
			for {
				time.Sleep(time.Second / 10)
				animationFrame <- true
			}
		}()
	}

	var state opgraph.CallGraph
	var loadingSpinner opgraph.DotLoadingSpinner
	output := opgraph.NewOutputManager()

	defer func() {
		output.Print(state.String(opFormatter, loadingSpinner, time.Now(), false))
		fmt.Println()
	}()

	clearGraph := func() {
		if displayLiveGraph {
			output.Clear()
		}
	}

	displayGraph := func() {
		if displayLiveGraph {
			output.Print(state.String(opFormatter, loadingSpinner, time.Now(), true))
		}
	}

	// listen for op end on a channel
	done := make(chan runResults, 1)
	go func() {
		dataResolver := dataresolver.New(
			cliParamSatisfier,
			node,
		)

		callID := "root"

		opHandle, err := dataResolver.Resolve(
			ctx,
			eventChannel,
			callID,
			opRef,
		)
		if err != nil {
			done <- runResults{
				err: err,
			}
			return
		}

		opFileReader, err := opHandle.GetContent(
			ctx,
			eventChannel,
			callID,
			opfile.FileName,
		)
		if err != nil {
			done <- runResults{
				err: err,
			}
			return
		}

		opFileBytes, err := ioutil.ReadAll(opFileReader)
		if nil != err {
			done <- runResults{
				err: err,
			}
			return
		}

		opFile, err := opfile.Unmarshal(
			filepath.Join(opHandle.Ref(), opfile.FileName),
			opFileBytes,
		)
		if err != nil {
			done <- runResults{
				err: err,
			}
			return
		}

		ymlFileInputSrc, err := cliParamSatisfier.NewYMLFileInputSrc(opts.ArgFile)
		if err != nil {
			done <- runResults{
				err: errors.Wrap(err, fmt.Sprintf("unable to load arg file at '%v'", opts.ArgFile)),
			}
			return
		}

		cliPromptInputSrc := cliParamSatisfier.NewCliPromptInputSrc(opFile.Inputs)
		if err != nil {
			done <- runResults{
				err: err,
			}
			return
		}
		argsMap, err := cliParamSatisfier.Satisfy(
			cliparamsatisfier.NewInputSourcer(
				cliParamSatisfier.NewSliceInputSrc(opts.Args, "="),
				ymlFileInputSrc,
				cliParamSatisfier.NewEnvVarInputSrc(),
				cliParamSatisfier.NewParamDefaultInputSrc(opFile.Inputs),
				cliPromptInputSrc,
			),
			opFile.Inputs,
		)
		if err != nil {
			done <- runResults{
				err: err,
			}
			return
		}

		outputs, err := node.StartOp(
			ctx,
			eventChannel,
			model.StartOpReq{
				Args: argsMap,
				Op: model.StartOpReqOp{
					Ref: opHandle.Ref(),
				},
			},
		)
		done <- runResults{
			outputs: outputs,
			err:     err,
		}
	}()

	for {
		select {
		case <-sigIntChannel:
			clearGraph()
			if !aSigIntWasReceivedAlready {
				cliOutput.Warning("Gracefully stopping... (signal Control-C again to force)")
				aSigIntWasReceivedAlready = true
				// events will continue to stream in, make sure we continue to display the graph while this happens
				displayGraph()
				cancel()
			} else {
				return nil, &RunError{
					ExitCode: 130,
					message:  "Terminated by Control-C",
				}
			}

		case <-sigInfoChannel:
			clearGraph()
			// clear two more lines
			fmt.Print("\033[1A\033[K\033[1A\033[K")
			fmt.Println(state.String(opFormatter, opgraph.StaticLoadingSpinner{}, time.Now(), false))
			displayGraph()

		case <-sigTermChannel:
			clearGraph()
			cliOutput.Error("Gracefully stopping...")
			displayGraph()
			cancel()

		case results := <-done:
			clearGraph()
			if results.err != nil && !errors.Is(results.err, context.Canceled) {
				return nil, results.err
			}
			return results.outputs, nil

		case event, isEventChannelOpen := <-eventChannel:
			clearGraph()
			if !isEventChannelOpen {
				return nil, errors.New("Event channel closed unexpectedly")
			}

			if err := state.HandleEvent(&event); err != nil {
				cliOutput.Error(fmt.Sprintf("%v", err))
			}

			cliOutput.Event(&event)
			displayGraph()
		case <-animationFrame:
			clearGraph()
			displayGraph()
		}
	}
}
