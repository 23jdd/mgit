package main

import (
	"fmt"
	"strings"
)

func validateRefPart(part string) error {
	if part == "" || part == "." || part == ".." {
		return fmt.Errorf("无效引用片段：%q", part)
	}
	if strings.ContainsAny(part, `\:*?"<>| `) {
		return fmt.Errorf("引用名不能包含特殊字符或空格：%q", part)
	}
	return nil
}

func selectedCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}
