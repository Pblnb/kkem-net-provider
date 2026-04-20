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

const (
	huaweiCloudVpcepNotFoundErrorCode = "EndPoint.0005"
)

// BoolPtr returns a pointer to a bool value.
func BoolPtr(b bool) *bool {
	return &b
}

// IsVpcepNotFoundError 检查错误是否是华为云 VPCEP 服务的 not-found 错误。判断标准是 404 状态码 + "EndPoint.0005" 错误代码
func IsVpcepNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var serviceErr *sdkerr.ServiceResponseError
	if errors.As(err, &serviceErr) {
		return serviceErr.StatusCode == http.StatusNotFound && serviceErr.ErrorCode == huaweiCloudVpcepNotFoundErrorCode
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
