// docker build -t my_docker . && docker run --network="host" --cap-add="SYS_ADMIN" my_docker run library/alpine /usr/local/bin/docker-explorer echo hey

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func main() {
	var err error
	log.Println(strings.Join(os.Args[0:len(os.Args)], " "))

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]
	commandDirPath := getCommandDirPath(command)
	
	// create directories to host the executable
	createDirectories(commandDirPath)
	

	// copy the executable to the temp directory,
	// and make it executable
	copyFile(command, "temp"+command)
	makeExecutable("temp" + command)

	// change into temp directory
	err = os.Chdir("./temp")
	if err != nil {
		log.Fatal(err)
	}

	// Fetch an image from the Docker Registry, uncompress it in the local directory
	imageName := os.Args[2]
	getImage(imageName)

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

	// request a new PID namespace
	// http://man7.org/linux/man-pages/man7/pid_namespaces.7.html
	// https://medium.com/@teddyking/namespaces-in-go-basics-e3f0fc1ff69a
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID,
	}

	// run the command
	if err := cmd.Run(); err != nil {
		// if there's an error, print the command's output
		// and exit with the same error code
		if exitError, ok := err.(*exec.ExitError); ok {
			fmt.Print(outbuf.String())
			var waitStatus syscall.WaitStatus
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			os.Exit(waitStatus.ExitStatus())
		}
	}
	// print stdout and stderr
	fmt.Print(outbuf.String())
	fmt.Fprintf(os.Stderr, errbuf.String())
}

func makeExecutable(path string) {
	var err error
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
	destination, err = os.Create(destinationFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		log.Fatal(err)
	}
}

func getImage(imageName string) {
	type AuthResponse struct {
		Token       string    `json:"token"`
		AccessToken string    `json:"access_token"`
		ExpiresIn   int       `json:"expires_in"`
		IssuedAt    time.Time `json:"issued_at"`
	}

	type ManifestResponse struct {
		FsLayers []struct {
			BlobSum string `json:"blobSum"`
		} `json:"fsLayers"`
	}
	
	imageAndTag := strings.Split(imageName, ":")
	image := imageAndTag[0]
	var tag string
	// if imageName isn't of image:tag form, 
	// set tag as it's needed in the url later
	if len(imageAndTag) > 1 {
		tag = imageAndTag[1]
	} else {
		tag = "latest"
	}
	
	// add librairy in front of the image name
	// for docker published images. This assumes the script will
	// always be called with such images names.
	authUrl := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/" + image + ":pull"
	client := &http.Client{}
	req, err := http.NewRequest("GET", authUrl, nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body) // response body is []byte
	var result AuthResponse
	if err := json.Unmarshal(body, &result); err != nil { // Parse []byte to go struct pointer
		fmt.Println("Can not unmarshal JSON")
	}

	layersUrl := "https://registry.hub.docker.com/v2/library/" + image + "/manifests/" + tag
	// Create a new request using http
	req, err = http.NewRequest("GET", layersUrl, nil)
	// Create a Bearer string by appending string access token
	var bearer = "Bearer " + result.AccessToken
	// add authorization header to the req
	req.Header.Add("Authorization", bearer)
	resp, err = client.Do(req)
	if err != nil {
		log.Println("Error on response.\n[ERROR] -", err)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error while reading the response bytes:", err)
	}
	log.Println("response " + string([]byte(body)))
	var manifest ManifestResponse
	if err := json.Unmarshal(body, &manifest); err != nil { // Parse []byte to go struct pointer
		fmt.Println("Can not unmarshal JSON")
	}
	if err != nil {
		log.Println("Error while reading the response bytes:", err)
	}
	// the manifest contains an FsLayers array
	// with the blobsum hash that points 
	// to the image layer to request
	for i := 0; i < len(manifest.FsLayers); i++ {
		manifestUrl := "https://registry.hub.docker.com/v2/library/" + image + "/blobs/" + manifest.FsLayers[i].BlobSum
		req, err = http.NewRequest("GET", manifestUrl, nil)
		req.Header.Add("Authorization", bearer)
		resp, err = client.Do(req)
		if err != nil {
			log.Println("Error on response.\n[ERROR] -", err)
		}
		defer resp.Body.Close()

		writeToFile(resp.Body, "./layersFile")

		var waitStatus syscall.WaitStatus

		cmd := exec.Command("tar", "--extract", "--file", "./layersFile", "-C", "./")
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
		os. Remove("./layersFile")
	}
}
func getCommandDirPath(command string) string {
	commandPathList := strings.Split(command, "/")
	commandDirList := commandPathList[:len(commandPathList)-1]
	if len(commandDirList) > 0 {
		return strings.Join(commandDirList, "/")
	} else {
		return commandDirList[0]
	}
}

func createDirectories(commandDirPath string) {
	var err error
	err = os.MkdirAll("./temp"+commandDirPath, 0777)
	if err != nil {
		log.Fatal(err)
	}
	// create directory for dev/null
	// https://rohitpaulk.com/articles/cmd-run-dev-null
	err = os.MkdirAll("./temp/dev/null/", 0777)
	if err != nil {
		log.Fatal(err)
	}
}

func writeToFile(content io.Reader, filename string) {
	// Create the file
	file, err := os.Create(filename)
	if err != nil {
		log.Println("Error creating the file", err)
	}
	defer file.Close()
	// Write the body to file
	_, err = io.Copy(file, content)
	if err != nil {
		log.Println("Error writing to the file", err)
	}
}