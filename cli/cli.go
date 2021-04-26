package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/appdataspec/sdk-golang/appdatapath"
	mow "github.com/jawher/mow.cli"
	"github.com/opctl/opctl/cli/internal/clicolorer"
	"github.com/opctl/opctl/cli/internal/clioutput"
	"github.com/opctl/opctl/cli/internal/cliparamsatisfier"
	"github.com/opctl/opctl/cli/internal/dataresolver"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/core"
	"github.com/opctl/opctl/sdks/go/node/core/containerruntime"
	"github.com/opctl/opctl/sdks/go/node/core/containerruntime/docker"
	"github.com/opctl/opctl/sdks/go/node/core/containerruntime/k8s"
	"github.com/opctl/opctl/sdks/go/opspec"
	"golang.org/x/term"
)

var testModeEnvVar = "OPCTL_TEST_MODE"

type cli interface {
	Run(args []string) error
}

func newCli(
	ctx context.Context,
) (cli, error) {
	cli := mow.App(
		"opctl",
		"Opctl is a free and open source distributed operation control system.",
	)
	cli.Version("v version", version)

	perUserAppDataPath, err := appdatapath.New().PerUser()
	if nil != err {
		return nil, err
	}

	datadirPath := cli.String(
		mow.StringOpt{
			Desc:   "Path of dir used to store opctl data",
			EnvVar: "OPCTL_DATA_DIR",
			Name:   "data-dir",
			Value:  filepath.Join(perUserAppDataPath, "miniopctl"),
		},
	)

	opFormatter := clioutput.NewCliOpFormatter(*datadirPath)

	cliOutput, err := clioutput.New(clicolorer.New(), opFormatter, os.Stderr, os.Stdout)
	if err != nil {
		return nil, err
	}

	exitWith := func(successMessage string, err error) {
		if err != nil {
			msg := err.Error()
			if msg != "" {
				cliOutput.Error(msg)
			}
			if re, ok := err.(*RunError); ok {
				mow.Exit(re.ExitCode)
			} else {
				mow.Exit(1)
			}
		}

		if successMessage != "" {
			cliOutput.Success(successMessage)
		}
		// Don't exit here with .Exit to allow container cleanup to happen
	}

	cliParamSatisfier := cliparamsatisfier.New(cliOutput)

	containerRuntime := cli.String(
		mow.StringOpt{
			Desc:   "Runtime for opctl containers",
			EnvVar: "OPCTL_CONTAINER_RUNTIME",
			Name:   "container-runtime",
			Value:  "docker",
		},
	)

	noColor := cli.BoolOpt("nc no-color", false, "Disable output coloring")

	cli.Before = func() {
		if *noColor {
			cliOutput.DisableColor()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	cli.After = func() {
		cancel()
	}

	var cr containerruntime.ContainerRuntime
	if "k8s" == *containerRuntime {
		cr, err = k8s.New()
	} else {
		cr, err = docker.New(ctx)
	}
	if nil != err {
		return nil, err
	}

	eventChannel := make(chan model.Event)

	opNode, err := core.New(ctx, cr, *datadirPath, eventChannel)
	if err != nil {
		return nil, err
	}

	cli.Command("auth", "Manage auth for OCI image registries", func(authCmd *mow.Cmd) {
		authCmd.Command("add", "Add auth for an OCI image registry", func(addCmd *mow.Cmd) {
			addCmd.Spec = "RESOURCES [ -u=<username> ] [ -p=<password> ]"

			resources := addCmd.StringArg("RESOURCES", "", "Resources this auth applies to in the form of a host or host/path (e.g. docker.io)")
			username := addCmd.StringOpt("u username", "", "Username")
			password := addCmd.StringOpt("p password", "", "Password")

			addCmd.Action = func() {
				exitWith(
					"",
					auth(
						ctx,
						opNode,
						model.AddAuthReq{
							Resources: *resources,
							Creds: model.Creds{
								Username: *username,
								Password: *password,
							},
						},
					),
				)
			}
		})
	})

	cli.Command("ls", "List operations", func(lsCmd *mow.Cmd) {
		const dirRefArgName = "DIR_REF"
		lsCmd.Spec = fmt.Sprintf("[%v]", dirRefArgName)
		dirRef := lsCmd.StringArg(dirRefArgName, opspec.DotOpspecDirName, "Reference to dir ops will be listed from")

		lsCmd.Action = func() {
			exitWith("", ls(ctx, cliParamSatisfier, opNode, *dirRef))
		}
	})

	cli.Command("op", "Manage ops", func(opCmd *mow.Cmd) {
		dataResolver := dataresolver.New(
			cliParamSatisfier,
			opNode,
		)

		opCmd.Command("create", "Create an op", func(createCmd *mow.Cmd) {
			path := createCmd.StringOpt("path", opspec.DotOpspecDirName, "Path the op will be created at")
			description := createCmd.StringOpt("d description", "", "Op description")
			name := createCmd.StringArg("NAME", "", "Op name")

			createCmd.Action = func() {
				exitWith(
					"",
					opspec.Create(
						filepath.Join(*path, *name),
						*name,
						*description,
					),
				)
			}
		})

		opCmd.Command("install", "Install an op", func(installCmd *mow.Cmd) {
			path := installCmd.StringOpt("path", opspec.DotOpspecDirName, "Path the op will be installed at")
			opRef := installCmd.StringArg("OP_REF", "", "Op reference (either `relative/path`, `/absolute/path`, `host/path/repo#tag`, or `host/path/repo#tag/path`)")

			installCmd.Action = func() {
				exitWith(
					"",
					opInstall(
						ctx,
						dataResolver,
						*opRef,
						*path,
					),
				)
			}
		})

		opCmd.Command("validate", "Validate an op", func(validateCmd *mow.Cmd) {
			opRef := validateCmd.StringArg("OP_REF", "", "Op reference (either `relative/path`, `/absolute/path`, `host/path/repo#tag`, or `host/path/repo#tag/path`)")

			validateCmd.Action = func() {
				exitWith(
					fmt.Sprintf("%v is valid", *opRef),
					opValidate(
						ctx,
						dataResolver,
						*opRef,
					),
				)
			}
		})
	})

	cli.Command("run", "Start and wait on an op", func(runCmd *mow.Cmd) {
		args := runCmd.StringsOpt("a", []string{}, "Explicitly pass args to op in format `-a NAME1=VALUE1 -a NAME2=VALUE2`")
		argFile := runCmd.StringOpt("arg-file", filepath.Join(opspec.DotOpspecDirName, "args.yml"), "Read in a file of args in yml format")
		defaultDisplayLiveGraph := term.IsTerminal(int(os.Stdout.Fd()))
		displayLiveGraph := runCmd.BoolOpt(
			"live-graph",
			defaultDisplayLiveGraph,
			"Display a live call graph for the op",
		)
		opRef := runCmd.StringArg("OP_REF", "", "Op reference (either `relative/path`, `/absolute/path`, `host/path/repo#tag`, or `host/path/repo#tag/path`)")

		runCmd.Action = func() {
			outputs, err := run(
				ctx,
				cliOutput,
				cliParamSatisfier,
				eventChannel,
				opNode,
				opFormatter,
				*opRef,
				&RunOpts{Args: *args, ArgFile: *argFile},
				*displayLiveGraph,
			)
			if err != nil {
				exitWith("", err)
			} else if len(outputs) > 0 {
				exitWith(model.FormatValueMap(outputs))
			} else {
				exitWith("", nil)
			}
		}
	})

	cli.Command("self-update", "Update opctl", func(selfUpdateCmd *mow.Cmd) {
		selfUpdateCmd.Action = func() {
			exitWith(selfUpdate())
		}
	})

	return cli, nil
}
