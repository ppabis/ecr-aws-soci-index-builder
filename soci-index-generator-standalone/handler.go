// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"errors"
	"path"

	"github.com/aws-ia/cfn-aws-soci-index-builder/soci-index-generator-lambda/utils/fs"
	"github.com/aws-ia/cfn-aws-soci-index-builder/soci-index-generator-lambda/utils/log"
	registryutils "github.com/aws-ia/cfn-aws-soci-index-builder/soci-index-generator-lambda/utils/registry"
	"github.com/containerd/containerd/images"
	"oras.land/oras-go/v2/content/oci"

	"github.com/awslabs/soci-snapshotter/soci"
	"github.com/awslabs/soci-snapshotter/soci/store"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/platforms"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TODO: Remove this once the SOCI library exports this error.
var (
	ErrEmptyIndex = errors.New("no ztocs created, all layers either skipped or produced errors")
)

const (
	BuildFailedMessage          = "SOCI index build error"
	PushFailedMessage           = "SOCI index push error"
	SkipPushOnEmptyIndexMessage = "Skipping pushing SOCI index as it does not contain any zTOCs"
	BuildAndPushSuccessMessage  = "Successfully built and pushed SOCI index"

	artifactsStoreName = "store"
	artifactsDbName    = "artifacts.db"
)

func handleRequest(ctx context.Context, imageUrl string, minLayerSize int64) (string, error) {
	digest := strings.Split(imageUrl, ":")[1]
	registryHost := strings.Split(imageUrl, "/")[0]
	repo := strings.TrimPrefix(imageUrl, registryHost+"/")
	repo = strings.TrimSuffix(repo, ":"+digest)

	ctx = context.WithValue(ctx, "RegistryURL", registryHost)

	registry, err := registryutils.Init(ctx, registryHost)
	if err != nil {
		fmt.Printf("Error initializing registry: %v", err)
	}

	err = registry.ValidateImageManifest(ctx, repo, digest)
	if err != nil {
		log.Warn(ctx, fmt.Sprintf("Image manifest validation error: %v", err))
		// Returning a non error to skip retries
		return "Exited early due to manifest validation error", nil
	}

	// Directory in lambda storage to store images and SOCI artifacts
	dataDir, err := createTempDir(ctx)
	if err != nil {
		return lambdaError(ctx, "Directory create error", err)
	}
	defer cleanUp(ctx, dataDir)

	// The channel to signal the deadline monitor goroutine to exit early
	quitChannel := make(chan int)
	defer func() {
		quitChannel <- 1
	}()

	setDeadline(ctx, quitChannel, dataDir)

	sociStore, err := initSociStore(ctx, dataDir)
	if err != nil {
		return lambdaError(ctx, "OCI storage initialization error", err)
	}

	desc, err := registry.Pull(ctx, repo, sociStore, digest)
	if err != nil {
		return lambdaError(ctx, "Image pull error", err)
	}

	image := images.Image{
		Name:   repo + "@" + digest,
		Target: *desc,
	}

	indexDescriptor, err := buildIndex(ctx, dataDir, sociStore, image, minLayerSize)
	if err != nil {
		if err.Error() == ErrEmptyIndex.Error() {
			log.Warn(ctx, SkipPushOnEmptyIndexMessage)
			return SkipPushOnEmptyIndexMessage, nil
		}
		return lambdaError(ctx, BuildFailedMessage, err)
	}
	ctx = context.WithValue(ctx, "SOCIIndexDigest", indexDescriptor.Digest.String())

	err = registry.Push(ctx, sociStore, *indexDescriptor, repo)
	if err != nil {
		return lambdaError(ctx, PushFailedMessage, err)
	}

	log.Info(ctx, BuildAndPushSuccessMessage)
	return BuildAndPushSuccessMessage, nil
}

// Create a temp directory in /tmp
// The directory is prefixed by the Lambda's request id
func createTempDir(ctx context.Context) (string, error) {
	// free space in bytes
	freeSpace := fs.CalculateFreeSpace("/tmp")
	log.Info(ctx, fmt.Sprintf("There are %d bytes of free space in /tmp directory", freeSpace))
	if freeSpace < 6_000_000_000 {
		// this is problematic because we support images as big as 6GB
		log.Warn(ctx, fmt.Sprintf("Free space in /tmp is only %d bytes, which is less than 6GB", freeSpace))
	}

	log.Info(ctx, "Creating a directory to store images and SOCI artifacts")
	tempDir, err := os.MkdirTemp("/tmp", "soci-lambda") // The temp dir name is prefixed by the request id
	return tempDir, err
}

// Clean up the data written by the Lambda
func cleanUp(ctx context.Context, dataDir string) {
	log.Info(ctx, fmt.Sprintf("Removing all files in %s", dataDir))
	if err := os.RemoveAll(dataDir); err != nil {
		log.Error(ctx, "Clean up error", err)
	}
}

// Set up deadline for the lambda to proactively clean up its data before the invocation timeout. We don't
// want to keep data in storage when the Lambda reaches its invocation timeout.
// This function creates a goroutine that will do cleanup when the invocation timeout is near.
// quitChannel is used for signaling that goroutine when the invocation ends naturally.
func setDeadline(ctx context.Context, quitChannel chan int, dataDir string) {
	// setting deadline as 10 seconds before lambda timeout.
	// reference: https://docs.aws.amazon.com/lambda/latest/dg/golang-context.html
	deadline, _ := ctx.Deadline()
	deadline = deadline.Add(-10 * time.Second)
	timeoutChannel := time.After(time.Until(deadline))
	go func() {
		for {
			select {
			case <-timeoutChannel:
				cleanUp(ctx, dataDir)
				log.Error(ctx, "Invocation timeout error", fmt.Errorf("Invocation timeout after 14 minutes and 50 seconds"))
				return
			case <-quitChannel:
				return
			}
		}
	}()
}

// Init containerd store
func initContainerdStore(dataDir string) (content.Store, error) {
	containerdStore, err := local.NewStore(path.Join(dataDir, artifactsStoreName))
	return containerdStore, err
}

// Init SOCI artifact store
func initSociStore(ctx context.Context, dataDir string) (*store.SociStore, error) {
	// Note: We are wrapping an *oci.Store in a store.SociStore because soci.WriteSociIndex
	// expects a store.Store, an interface that extends the oci.Store to provide support
	// for garbage collection.
	ociStore, err := oci.NewWithContext(ctx, path.Join(dataDir, artifactsStoreName))
	return &store.SociStore{ociStore}, err
}

// Init a new instance of SOCI artifacts DB
func initSociArtifactsDb(dataDir string) (*soci.ArtifactsDb, error) {
	artifactsDbPath := path.Join(dataDir, artifactsDbName)
	artifactsDb, err := soci.NewDB(artifactsDbPath)
	if err != nil {
		return nil, err
	}
	return artifactsDb, nil
}

// Build soci index for an image and returns its ocispec.Descriptor
func buildIndex(ctx context.Context, dataDir string, sociStore *store.SociStore, image images.Image, minLayerSize int64) (*ocispec.Descriptor, error) {
	log.Info(ctx, "Building SOCI index")
	platform := platforms.DefaultSpec() // TODO: make this a user option

	artifactsDb, err := initSociArtifactsDb(dataDir)
	if err != nil {
		return nil, err
	}

	containerdStore, err := initContainerdStore(dataDir)
	if err != nil {
		return nil, err
	}

	builder, err := soci.NewIndexBuilder(containerdStore, sociStore, artifactsDb, soci.WithPlatform(platform), soci.WithMinLayerSize(minLayerSize))
	if err != nil {
		return nil, err
	}

	// Build the SOCI index
	index, err := builder.Build(ctx, image)
	if err != nil {
		return nil, err
	}

	// Write the SOCI index to the OCI store
	err = soci.WriteSociIndex(ctx, index, sociStore, artifactsDb)
	if err != nil {
		return nil, err
	}

	// Get SOCI indices for the image from the OCI store
	// TODO: consider making soci's WriteSociIndex to return the descriptor directly
	indexDescriptorInfos, _, err := soci.GetIndexDescriptorCollection(ctx, containerdStore, artifactsDb, image, []ocispec.Platform{platform})
	if err != nil {
		return nil, err
	}
	if len(indexDescriptorInfos) == 0 {
		return nil, errors.New("No SOCI indices found in OCI store")
	}
	sort.Slice(indexDescriptorInfos, func(i, j int) bool {
		return indexDescriptorInfos[i].CreatedAt.Before(indexDescriptorInfos[j].CreatedAt)
	})

	return &indexDescriptorInfos[len(indexDescriptorInfos)-1].Descriptor, nil
}

// Log and return the lambda handler error
func lambdaError(ctx context.Context, msg string, err error) (string, error) {
	log.Error(ctx, msg, err)
	return msg, err
}
