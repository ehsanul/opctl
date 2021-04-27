package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/opctl/opctl/cli/internal/cliparamsatisfier"
	"github.com/opctl/opctl/cli/internal/dataresolver"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/core"
	"github.com/opctl/opctl/sdks/go/opspec"
)

// ls implements "ls" command
func ls(
	ctx context.Context,
	cliParamSatisfier cliparamsatisfier.CLIParamSatisfier,
	node core.Core,
	dirRef string,
) error {
	dataResolver := dataresolver.New(
		cliParamSatisfier,
		node,
	)

	_tabWriter := new(tabwriter.Writer)
	defer _tabWriter.Flush()
	_tabWriter.Init(os.Stdout, 0, 8, 1, '\t', 0)

	fmt.Fprintln(_tabWriter, "REF\tDESCRIPTION")

	eventChannel := make(chan model.Event)
	callID := ""

	dirHandle, err := dataResolver.Resolve(
		ctx,
		eventChannel,
		callID,
		dirRef,
	)
	if err != nil {
		return err
	}

	opsByPath, err := opspec.List(
		ctx,
		eventChannel,
		callID,
		dirHandle,
	)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	for path, op := range opsByPath {
		opRef := filepath.Join(dirHandle.Ref(), path)
		if filepath.IsAbs(opRef) {
			// make absolute paths relative
			relOpRef, err := filepath.Rel(cwd, opRef)
			if err != nil {
				return err
			}

			opRef = strings.TrimPrefix(relOpRef, ".opspec/")
		}

		scanner := bufio.NewScanner(strings.NewReader(op.Description))
		if scanner.Scan() {
			// first line of description, add the op ref
			fmt.Fprintf(_tabWriter, "%v\t%v", opRef, scanner.Text())
		}
		for scanner.Scan() {
			// subsequent lines, don't add the op ref but let the description span multiple lines
			fmt.Fprintf(_tabWriter, "\n\t%v", scanner.Text())
		}
		fmt.Fprintln(_tabWriter)
	}

	return nil
}
