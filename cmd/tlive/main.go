package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tlive <command> [args...]")
		os.Exit(1)
	}
	fmt.Printf("TermLive: would run %v\n", os.Args[1:])
}
