package clioutput

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// OpFormatter formats an op ref in some way
type OpFormatter interface {
	FormatOpRef(opRef string) string
}

// CliOpFormatter formats an op ref in the context of a CLI run
type CliOpFormatter struct {
	datadirPath string
}

// NewCliOpFormatter creates a new CliOpFormatter
func NewCliOpFormatter(datadirPath string) CliOpFormatter {
	return CliOpFormatter{datadirPath: datadirPath}
}

// FormatOpRef gives a more appropriate description of an op's reference
// Local ops will be formatted as paths relative to the working directory or
// home directory, installed ops will be formatted as url-like op refs
func (of CliOpFormatter) FormatOpRef(opRef string) string {
	if path.IsAbs(opRef) {
		cwd, err := os.Getwd()
		if err != nil {
			return opRef
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return opRef
		}
		dataDirPath := of.datadirPath
		if strings.HasPrefix(opRef, dataDirPath) {
			return opRef[len(filepath.Join(dataDirPath, "ops")+string(os.PathListSeparator)):]
		}
		if strings.HasPrefix(opRef, cwd) {
			return "." + opRef[len(cwd):]
		}
		if strings.HasPrefix(opRef, home) {
			return "~" + opRef[len(home):]
		}
	}
	return opRef
}

// SimpleOpFormatter just mirrors the op ref as is
type SimpleOpFormatter struct{}

// FormatOpRef returns the op ref
func (SimpleOpFormatter) FormatOpRef(opRef string) string {
	return opRef
}
