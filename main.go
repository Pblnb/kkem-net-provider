package main

import (
	"context"
	"flag"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"huawei.com/kkem/kkem-net-provider/internal/provider"
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
