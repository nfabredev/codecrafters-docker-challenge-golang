package main

import (
	"bytes"
	"fmt"
	"io"
	// "io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]
	var waitStatus syscall.WaitStatus

	var err error

	// create directories to host the executable
	err = os.MkdirAll("./temp/usr/local/bin/", 0777)
	if err != nil {
		log.Fatal(err)
	}
	// create directory for dev/null
	// https://rohitpaulk.com/articles/cmd-run-dev-null
	err = os.MkdirAll("./temp/dev/null/", 0777)
	if err != nil {
		log.Fatal(err)
	}

	// copy the executable to the temp directory,
	// and make it executable
	copyFile(command, "temp" + command)
	makeExecutable("temp" + command)

	// change into temp directory
	err = os.Chdir("./temp"); 
	if err != nil {
		log.Fatal(err)
	}
	// make the current directory the chroot jail
	err = syscall.Chroot(".")
	if err != nil {
		log.Fatal(err)
	}

	// prepare the command for execution
	cmd := exec.Command(command, args...)
	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	// run the command
	if err := cmd.Run(); err != nil {
		// if there's an error, print the command's output
		// and exit with the same error code
		if exitError, ok := err.(*exec.ExitError); ok {
			fmt.Print(outbuf.String())
			// this doesn't actually print anything
			// fmt.Fprintf(os.Stderr, errbuf.String())
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			os.Exit(waitStatus.ExitStatus())
		}
	}
	// print stdout and stderr
	fmt.Print(outbuf.String())
	fmt.Fprintf(os.Stderr, errbuf.String())

	// cleanup()
}

// make program executable
func makeExecutable(path string) {
	var err error
	// get file info
	// _, err := os.Stat(path)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// change mode
	err = os.Chmod(path, 0777)
	if err != nil {
		log.Fatal(err)
	}
}

func copyFile(sourceFilePath, destinationFilePath string) {
	var err error
	var source *os.File
	source, err = os.Open(sourceFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	var destination *os.File
	// Create destination file
	destination, err = os.Create(destinationFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer destination.Close()

	// This will copy
	_, err = io.Copy(destination, source)
	if err != nil {
		log.Fatal(err)
	}
}

// func cleanup() {
// 	if err := os.Chdir("../"); err != nil {
// 		log.Fatal(err)
// 	}

// 	if err := os.RemoveAll("temp/"); err != nil {
// 		log.Fatal(err)
// 	}
// }

// list recursively files in a directory
// func listFilesRecursively(path string) {
// 	files, err := ioutil.ReadDir(path)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	for _, f := range files {
// 		fmt.Println(f.Name())
// 		if f.IsDir() {
// 			listFilesRecursively(path + "/" + f.Name())
// 		}
// 	}
// }