package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/pkg/errors"
)

// Pull pulls 'dataRef' to 'path'
//
// expected errs:
//  - ErrDataProviderAuthentication on authentication failure
//  - ErrDataProviderAuthorization on authorization failure
func (gp *_git) pull(
	ctx context.Context,
	dataRef *ref,
) error {
	opPath := dataRef.ToPath(gp.basePath)

	auth, err := ssh.NewSSHAgentAuth("git")
	if err != nil {
		return errors.Wrap(err, "failed to connect to ssh agent")
	}

	cloneOptions := &git.CloneOptions{
		URL:           fmt.Sprintf("ssh://git@%s", dataRef.Name),
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/tags/%s", dataRef.Version)),
		Depth:         1,
		Progress:      os.Stdout,
		Auth:          auth,
	}

	if _, err := git.PlainCloneContext(
		ctx,
		opPath,
		false,
		cloneOptions,
	); err != nil {
		if _, ok := err.(git.NoMatchingRefSpecError); ok {
			return fmt.Errorf("version '%s' not found", dataRef.Version)
		}
		if errors.Is(err, transport.ErrAuthenticationRequired) {
			return model.ErrDataProviderAuthentication{}
		}
		if errors.Is(err, transport.ErrAuthorizationFailed) {
			return model.ErrDataProviderAuthorization{}
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || err == ctx.Err() {
			fmt.Fprintf(os.Stderr, "cleaning up %v\n", dataRef)
			err := os.RemoveAll(opPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to cleanup partially downloaded op: %v\n", err)
			}
		}
		return err
	}

	return os.RemoveAll(filepath.Join(opPath, ".git"))
}
