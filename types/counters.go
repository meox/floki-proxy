// Copyright 2021 Gian Lorenzo Meocci (glmeocci@gmail.com). All rights reserved.

package types

import "go.uber.org/atomic"

type MethodCounters map[string]*atomic.Uint64
