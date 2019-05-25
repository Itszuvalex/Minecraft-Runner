package main

import (
	"fmt"
	"mcrunner"
)

func main() {
	fmt.Println("Hello world")
	runner := new(mcrunner.McRunner)
	runner.Start()
}
