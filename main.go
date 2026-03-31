package main

import (
	"context"
	"flag"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/kkem/terraform-provider-kkem/internal/provider"
)

func main() {
	flag.Parse()

	err := providerserver.Serve(
		context.Background(),
		provider.New,
		providerserver.ServeOpts{
			Address: "kkem.internal/net/kkem",
		},
	)

	if err != nil {
		panic(err)
	}
}
