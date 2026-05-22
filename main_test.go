/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/stretchr/testify/assert"
)

const testProviderAddress = "huawei.com/provider/kkem"

func Test_main(t *testing.T) {
	testCases := []struct {
		name          string
		serveErr      error
		expectedPanic string
	}{
		{
			name: "GIVEN provider server starts successfully WHEN main SHOULD return without panic",
		},
		{
			name:          "GIVEN provider server returns error WHEN main SHOULD log fatal error",
			serveErr:      errors.New("serve failed"),
			expectedPanic: "Failed to start provider server: serve failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			patches := gomonkey.NewPatches()
			defer patches.Reset()

			patches.ApplyFunc(providerserver.Serve, func(ctx context.Context, providerFunc func() provider.Provider,
				opts providerserver.ServeOpts) error {
				assert.NotNil(t, ctx)
				assert.NotNil(t, providerFunc)
				assert.Equal(t, testProviderAddress, opts.Address)
				return tc.serveErr
			})
			patches.ApplyFunc(log.Fatalf, func(format string, v ...any) {
				panic(fmt.Sprintf(format, v...))
			})

			if tc.expectedPanic == "" {
				assert.NotPanics(t, main)
			} else {
				assert.PanicsWithValue(t, tc.expectedPanic, main)
			}
		})
	}
}
