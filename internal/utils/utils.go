/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package utils

import (
	"errors"
	"net/http"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
)

// BoolPtr returns a pointer to a bool value.
func BoolPtr(b bool) *bool {
	return &b
}

// IsNotFoundError checks if the error is a 404 Not Found response from Huawei Cloud SDK.
// It uses type assertion to extract the ServiceResponseError and check the StatusCode.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var serviceErr *sdkerr.ServiceResponseError
	if errors.As(err, &serviceErr) {
		return serviceErr.StatusCode == http.StatusNotFound
	}

	return false
}
