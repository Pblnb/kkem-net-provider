/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 */

package service

import "time"

const (
	pollingInterval     = 5 * time.Second
	pollingTimeout      = 5 * time.Minute
	pollingErrTolerance = 3
)

// 重试相关常量
const (
	maxRetryCount  = 3
	retryBaseDelay = time.Second
)
