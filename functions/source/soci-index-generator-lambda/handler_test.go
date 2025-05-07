// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"testing"
	"time"
)

// This test ensures that the handler can pull Docker and OCI images, build, and push the SOCI index back to the repository.
// To run this test locally, you need to push an image to a private ECR repository, and set following environment variables:
// DOCKER_IMAGE_URI: the fully URI of your Docker image.
// OCI_IMAGE_URI: the fully URI of your OCI image.
func TestHandlerHappyPath(t *testing.T) {
	doTest := func(imageUri string) {
		ctx := context.Background()
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute*5))
		defer cancel()

		resp, err := handleRequest(ctx, imageUri, 10485760/4)
		if err != nil {
			t.Fatalf("HandleRequest failed %v", err)
		}

		expected_resp := "Successfully built and pushed SOCI index"
		if resp != expected_resp {
			t.Fatalf("Unexpected response. Expected %s but got %s", expected_resp, resp)
		}
	}

	doTest(os.Getenv("DOCKER_IMAGE_URI"))
	doTest(os.Getenv("OCI_IMAGE_URI"))
}

// This test ensures that the handler can validate the input digest media type
// To run this test locally, you need to set following environment variables:
// NOT_AN_IMAGE_URI: the fully URI of anything that isn't an image.
func TestHandlerInvalidDigestMediaType(t *testing.T) {
	imageUri := os.Getenv("NOT_AN_IMAGE_URI")

	ctx := context.Background()
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute))
	defer cancel()

	resp, err := handleRequest(ctx, imageUri, 10485760/4)
	if err != nil {
		t.Fatalf("Invalid image digest is not expected to fail")
	}

	expected_resp := "Exited early due to manifest validation error"
	if resp != expected_resp {
		t.Fatalf("Unexpected response. Expected %s but got %s", expected_resp, resp)
	}
}
