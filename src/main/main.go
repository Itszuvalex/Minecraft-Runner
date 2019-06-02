package main

import (
	"fmt"
	"mcrunner"
)

func main() {
	fmt.Println("Starting server...")
	runner := new(mcrunner.McRunner)
	runner.Start()
}
