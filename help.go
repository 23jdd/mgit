package main

import "fmt"

func printHelp() {
	fmt.Println("mgit - 用 Go 实现的迷你 Git")
	fmt.Println()
	fmt.Println("用法：")
	for _, cmd := range commands {
		for i, usage := range cmd.Usage {
			if i == 0 {
				fmt.Printf("  %s\n", usage)
				continue
			}
			fmt.Printf("  %s\n", usage)
		}
		fmt.Printf("      %s\n\n", cmd.Description)
	}
}
