/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package utils

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
)

// BoolPtr returns a pointer to a bool value.
func BoolPtr(b bool) *bool {
	return &b
}

// IsHuaweiCloudNotFoundError checks if the error is a 404 Not Found response from Huawei Cloud SDK.
// It uses type assertion to extract the ServiceResponseError and check the StatusCode.
func IsHuaweiCloudNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var serviceErr *sdkerr.ServiceResponseError
	if errors.As(err, &serviceErr) {
		return serviceErr.StatusCode == http.StatusNotFound
	}

	return false
}

// RetryWithBackoff retries the given operation up to maxRetries times with linear backoff.
// The wait interval between attempts is baseInterval * (attempt number), starting from 1.
// If the context is cancelled, it returns ctx.Err() immediately.
func RetryWithBackoff(ctx context.Context, maxRetries int, baseInterval time.Duration, operation func() error) error {
	var err error
	for i := range maxRetries {
		err = operation()
		if err == nil {
			return nil
		}
		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(baseInterval * time.Duration(i+1)):
			}
		}
	}
	return err
}
