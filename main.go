/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"huawei.com/kkem/kkem-net-provider/internal/provider"
)

func main() {
	const ProviderVersion = "0.1.0"

	err := providerserver.Serve(
		context.Background(),
		provider.NewKKEMProvider(ProviderVersion),
		providerserver.ServeOpts{
			Address: "huawei.com/provider/kkem",
		},
	)

	if err != nil {
		log.Fatalf("Failed to start provider server: %v", err)
	}
}
