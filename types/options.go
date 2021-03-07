// Copyright 2021 Gian Lorenzo Meocci (glmeocci@gmail.com). All rights reserved.

package types

import (
	"fmt"
	"strconv"
	"strings"
)

type FailingPrefixCode map[string]int

func (fp FailingPrefixCode) String() string {
	var rs []string
	for k, v := range fp {
		rs = append(rs, fmt.Sprintf("%s:%d", k, v))
	}

	return strings.Join(rs, ";")
}

func (fp *FailingPrefixCode) Set(x string) error {
	if x == "" {
		return nil
	}

	m := make(map[string]int)
	tks := strings.Split(x, ";")

	for _, e := range tks {
		pair := strings.Split(e, ":")
		if len(pair) != 2 {
			return fmt.Errorf("decoding %s", x)
		}
		code, err := strconv.Atoi(pair[1])
		if err != nil {
			return fmt.Errorf("canno convert %s to int: %w", pair[1], err)
		}
		m[pair[0]] = code
	}

	*fp = m
	return nil
}
