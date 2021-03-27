// Copyright 2021 Gian Lorenzo Meocci (glmeocci@gmail.com). All rights reserved.

package types

import (
	"fmt"
	"sync"
)

type MethodCounters struct {
	data map[string]uint64
	m    sync.Mutex
}

func NewMethodCounters() *MethodCounters {
	return &MethodCounters{
		data: make(map[string]uint64),
	}
}

func (mc *MethodCounters) Add(method string, v uint64) {
	mc.m.Lock()
	defer mc.m.Unlock()
	old := mc.data[method]
	mc.data[method] = old + v
}

func (mc *MethodCounters) PrintCounters() {
	mc.m.Lock()
	defer mc.m.Unlock()

	if len(mc.data) == 0 {
		return
	}

	fmt.Printf("Method Counters\n")
	for k, v := range mc.data {
		fmt.Printf("%s: %d\n", k, v)
	}
	fmt.Printf("\n")
}
