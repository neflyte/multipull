package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types"

	"github.com/docker/docker/client"
)

const (
	logOptions = log.LstdFlags | log.Lmsgprefix | log.Lshortfile
)

func main() {
	ctx := context.Background()
	logger := log.New(os.Stdout, "multipull] ", logOptions)
	if len(os.Args) <= 1 {
		logger.Fatal("no arguments specified")
	}
	logger.Printf("os.Args[1:]=%#v", os.Args[1:])
	logger.Println("initialize client")
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Fatalf("error initializing docker client: %s", err.Error())
	}
	for _, arg := range os.Args[1:] {
		logger.Println("pulling image...")
		reader, err := cli.ImagePull(ctx, arg, types.ImagePullOptions{})
		if err != nil {
			logger.Fatalf("error pulling image: %s", err.Error())
		}
		defer func() {
			err = reader.Close()
			if err != nil {
				logger.Printf("error closing reader: %s", err.Error())
			}
		}()
		logger.Println("copying result to stdout")
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			logger.Fatalf("error copying response to stdout: %s", err.Error())
		}
	}
	logger.Println("done.")
}
