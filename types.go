package main

import (
	"fmt"
	"strings"

	"github.com/23jdd/mgit/repo"
)

var myGitDir = repo.Dir()

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("值不能为空")
	}
	*s = append(*s, value)
	return nil
}
