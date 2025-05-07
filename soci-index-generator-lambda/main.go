package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"
)

func main() {
	// parse the repository URI from a -repository flag
	repo := flag.String("repository", "", "OCI repository URI (with tag or digest) to build the SOCI index for")
	minLayerSize := flag.Int64("min-layer-size", 10485760, "minimum layer size to build a ztoc for a layer (default 10MB)")
	flag.Parse()

	if *repo == "" {
		log.Fatal("missing required -repository argument")
	}

	ctx := context.Background()
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute*5))
	defer cancel()
	// invoke the handler with the provided repository URI
	out, err := handleRequest(ctx, *repo, *minLayerSize)
	if err != nil {
		log.Fatalf("error building SOCI index for %q: %v", *repo, err)
	}
	fmt.Println(out)
}
