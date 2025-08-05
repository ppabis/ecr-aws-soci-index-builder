// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/lambdacontext"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ExpectedResponse struct {
	MediaTypes []string // acceptable media types
	Config     ocispec.Descriptor
}

func TestHeadManifest(t *testing.T) {
	doTest := func(registryUrl string, repository string, digestOrTag string, expected ExpectedResponse) {
		// making the test context
		lc := lambdacontext.LambdaContext{}
		lc.AwsRequestID = "abcd-1234-test-head-manifest"
		ctx := lambdacontext.NewContext(context.Background(), &lc)
		registry, err := Init(ctx, registryUrl)
		if err != nil {
			panic(err)
		}

		descriptor, err := registry.HeadManifest(context.Background(), repository, digestOrTag)
		if err != nil {
			panic(err)
		}

		// Check if the media type matches any of the alternative types
		for _, mediaType := range expected.MediaTypes {
			if descriptor.MediaType == mediaType {
				return
			}
		}

		// If we get here, the media type didn't match any expected types
		t.Fatalf("Incorrect manifest media type of %s. Got %s but expected one of: %v",
			digestOrTag, descriptor.MediaType, expected.MediaTypes)
	}

	expected := ExpectedResponse{
		MediaTypes: []string{MediaTypeDockerManifestList, MediaTypeOCIImageIndex},
	}
	doTest("public.ecr.aws", "docker/library/redis", "7", expected)

	expected = ExpectedResponse{
		MediaTypes: []string{MediaTypeDockerManifestList, MediaTypeOCIImageIndex},
	}
	doTest("public.ecr.aws", "lambda/python", "3.10", expected)

	expected = ExpectedResponse{
		MediaTypes: []string{MediaTypeDockerManifest},
	}
	doTest("public.ecr.aws", "lambda/python", "3.10-x86_64", expected)

	expected = ExpectedResponse{
		MediaTypes: []string{MediaTypeDockerManifest},
	}
	doTest("docker.io", "library/redis", "sha256:afd1957d6b59bfff9615d7ec07001afb4eeea39eb341fc777c0caac3fcf52187", expected)
}

func TestGetManifest(t *testing.T) {
	doTest := func(registryUrl string, repository string, digestOrTag string, expected ExpectedResponse) {
		// making the test context
		lc := lambdacontext.LambdaContext{}
		lc.AwsRequestID = "abcd-1234-test-get-manifest"
		ctx := lambdacontext.NewContext(context.Background(), &lc)
		registry, err := Init(ctx, registryUrl)
		if err != nil {
			panic(err)
		}

		manifest, err := registry.GetManifest(context.Background(), repository, digestOrTag)
		if err != nil {
			panic(err)
		}

		// Check if the media type matches any of the expected types
		mediaTypeMatched := false
		for _, mediaType := range expected.MediaTypes {
			if manifest.MediaType == mediaType {
				mediaTypeMatched = true
				break
			}
		}

		if !mediaTypeMatched {
			t.Fatalf("Incorrect manifest media type of %s. Got %s but expected one of: %v",
				digestOrTag, manifest.MediaType, expected.MediaTypes)
		}

		if manifest.Config.MediaType != expected.Config.MediaType {
			t.Fatalf("Incorrect config's media type. Expected %s but got %s",
				expected.Config.MediaType, manifest.Config.MediaType)
		}
	}

	expected := ExpectedResponse{
		MediaTypes: []string{MediaTypeDockerManifestList, MediaTypeOCIImageIndex},
		Config: ocispec.Descriptor{
			MediaType: "",
		},
	}
	doTest("public.ecr.aws", "docker/library/redis", "7", expected)

	expected = ExpectedResponse{
		MediaTypes: []string{MediaTypeDockerManifestList, MediaTypeOCIImageIndex},
		Config: ocispec.Descriptor{
			MediaType: "",
		},
	}
	doTest("public.ecr.aws", "lambda/python", "3.10", expected)

	expected = ExpectedResponse{
		MediaTypes: []string{MediaTypeDockerManifest},
		Config: ocispec.Descriptor{
			MediaType: MediaTypeDockerImageConfig,
		},
	}
	doTest("public.ecr.aws", "lambda/python", "3.10-x86_64", expected)

	expected = ExpectedResponse{
		MediaTypes: []string{MediaTypeDockerManifest},
		Config: ocispec.Descriptor{
			MediaType: MediaTypeDockerImageConfig,
		},
	}
	doTest("docker.io", "library/redis", "sha256:afd1957d6b59bfff9615d7ec07001afb4eeea39eb341fc777c0caac3fcf52187", expected)
}
