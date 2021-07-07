package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	// fmt.Println("Your code goes here!")

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]
	var waitStatus syscall.WaitStatus

	cmd := exec.Command(command, args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			os.Exit(waitStatus.ExitStatus())
		}
	}
	fmt.Print(outb.String())
	fmt.Fprintf(os.Stderr, errb.String())
}
