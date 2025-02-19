package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerClientPkg "github.com/docker/docker/client"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/pubsub"
)

type runContainer interface {
	RunContainer(
		ctx context.Context,
		req *model.ContainerCall,
		rootCallID string,
		eventPublisher pubsub.EventPublisher,
		stdout io.WriteCloser,
		stderr io.WriteCloser,
	) (*int64, error)
}

func newRunContainer(
	ctx context.Context,
	dockerClient dockerClientPkg.CommonAPIClient,
) (runContainer, error) {
	hcf, err := newHostConfigFactory(ctx, dockerClient)
	if err != nil {
		return _runContainer{}, err
	}

	rc := _runContainer{
		containerStdErrStreamer: newContainerStdErrStreamer(dockerClient),
		containerStdOutStreamer: newContainerStdOutStreamer(dockerClient),
		dockerClient:            dockerClient,
		ensureNetworkExistser:   newEnsureNetworkExistser(dockerClient),
		hostConfigFactory:       hcf,
		imagePuller:             newImagePuller(dockerClient),
		imagePusher:             newImagePusher(),
	}
	return rc, nil
}

type _runContainer struct {
	containerStdErrStreamer containerLogStreamer
	containerStdOutStreamer containerLogStreamer
	dockerClient            dockerClientPkg.CommonAPIClient
	ensureNetworkExistser   ensureNetworkExistser
	hostConfigFactory       hostConfigFactory
	imagePuller             imagePuller
	imagePusher             imagePusher
}

func (cr _runContainer) RunContainer(
	ctx context.Context,
	req *model.ContainerCall,
	rootCallID string,
	eventPublisher pubsub.EventPublisher,
	stdout io.WriteCloser,
	stderr io.WriteCloser,
) (*int64, error) {
	defer stdout.Close()
	defer stderr.Close()

	// ensure user defined network exists to allow inter container resolution via name
	// @TODO: remove when socket outputs supported
	if err := cr.ensureNetworkExistser.EnsureNetworkExists(
		ctx,
		dockerNetworkName,
	); err != nil {
		return nil, err
	}

	// for docker, we prefix name with opctl_ in order to allow external tools to know it's an opctl managed container
	// do not change this prefix as it might break external consumers
	containerName := getContainerName(req.ContainerID)
	defer func() {
		// ensure container always cleaned up: gracefully stop then delete it
		// @TODO: consolidate logic with DeleteContainerIfExists
		newCtx := context.Background() // always use a fresh context, to clean up after cancellation
		stopTimeout := 3 * time.Second
		cr.dockerClient.ContainerStop(
			newCtx,
			containerName,
			&stopTimeout,
		)
		cr.dockerClient.ContainerRemove(
			newCtx,
			containerName,
			types.ContainerRemoveOptions{
				RemoveVolumes: true,
				Force:         true,
			},
		)
	}()

	var imageErr error
	if req.Image.Src != nil {
		imageRef := fmt.Sprintf("%s:latest", req.ContainerID)
		req.Image.Ref = &imageRef

		imageErr = cr.imagePusher.Push(
			ctx,
			imageRef,
			req.Image.Src,
		)
	} else {
		imageErr = cr.imagePuller.Pull(
			ctx,
			req,
			rootCallID,
			eventPublisher,
		)
		// don't err yet; image might be cached. We allow this to support offline use
	}

	portBindings, err := constructPortBindings(
		req.Ports,
	)
	if err != nil {
		return nil, err
	}

	hostConfig := cr.hostConfigFactory.Construct(
		req.Dirs,
		req.Files,
		req.Sockets,
		portBindings,
	)

	// construct networking config
	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			dockerNetworkName: {},
		},
	}
	if req.Name != nil {
		networkingConfig.EndpointsConfig[dockerNetworkName].Aliases = []string{
			*req.Name,
		}
	}

	// create container
	containerCreatedResponse, createErr := cr.dockerClient.ContainerCreate(
		ctx,
		constructContainerConfig(
			req.Cmd,
			req.EnvVars,
			*req.Image.Ref,
			portBindings,
			req.WorkDir,
		),
		hostConfig,
		networkingConfig,
		containerName,
	)
	if createErr != nil {
		select {
		case <-ctx.Done():
			// we got killed;
			return nil, nil
		default:
			if imageErr == nil {
				return nil, createErr
			}
			// if imageErr occurred prior; combine errors
			return nil, errors.New(strings.Join([]string{imageErr.Error(), createErr.Error()}, ", "))
		}
	}

	// start container
	if err := cr.dockerClient.ContainerStart(
		ctx,
		containerCreatedResponse.ID,
		types.ContainerStartOptions{},
	); err != nil {
		return nil, err
	}

	var waitGroup sync.WaitGroup
	errChan := make(chan error, 3)
	waitGroup.Add(2)

	go func() {
		if err := cr.containerStdErrStreamer.Stream(
			ctx,
			containerName,
			stderr,
		); err != nil {
			errChan <- err
		}
		waitGroup.Done()
	}()

	go func() {
		if err := cr.containerStdOutStreamer.Stream(
			ctx,
			containerName,
			stdout,
		); err != nil {
			errChan <- err
		}
		waitGroup.Done()
	}()

	var exitCode int64
	waitOkChan, waitErrChan := cr.dockerClient.ContainerWait(
		ctx,
		containerName,
		container.WaitConditionNotRunning,
	)
	select {
	case waitOk := <-waitOkChan:
		exitCode = waitOk.StatusCode
	case waitErr := <-waitErrChan:
		err = fmt.Errorf("error waiting on container: %w", waitErr)
	}

	// ensure stdout, and stderr all read before returning
	waitGroup.Wait()

	if err != nil && len(errChan) > 0 {
		// non-destructively set err
		err = <-errChan
	}
	return &exitCode, err

}
