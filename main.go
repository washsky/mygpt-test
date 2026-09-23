package main

import (
	"fmt"
	"runtime"
)

var version = "dev"

func main() {
	fmt.Println("mygpt-test")
	fmt.Println("version:", version)
	fmt.Println("go:", runtime.Version())
	fmt.Println("os/arch:", runtime.GOOS, runtime.GOARCH)
}
