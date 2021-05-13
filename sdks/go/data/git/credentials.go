package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/opctl/opctl/sdks/go/model"
)

// NOTE: This requires a credential helper configuration that does not require
// user interaction. It does not currently support prompting the user for
// manual creds.
func getCredentials(ctx context.Context, url string) (model.Creds, error) {
	// not supported in go-git
	// https://github.com/go-git/go-git/issues/250
	// this is a bummer, since it means this is very difficult to test

	cmd := exec.CommandContext(ctx, "git", "credential", "fill")
	in, err := cmd.StdinPipe()
	if err != nil {
		return model.Creds{}, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return model.Creds{}, err
	}

	if err := cmd.Start(); err != nil {
		return model.Creds{}, err
	}

	fmt.Fprintf(in, "url=%s\n", url)
	in.Close()

	var creds model.Creds
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return model.Creds{}, errors.New("failed to parse incoming line")
		}
		switch parts[0] {
		case "username":
			creds.Username = parts[1]
		case "password":
			creds.Password = parts[1]
		}
	}

	if creds.Username == "" {
		return creds, errors.New("failed to get username from git credential")
	}
	if creds.Password == "" {
		return creds, errors.New("failed to get password from git credential")
	}

	if err := cmd.Wait(); err != nil {
		return model.Creds{}, err
	}

	return creds, nil
}
