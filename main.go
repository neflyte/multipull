package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/panjf2000/ants/v2"
)

const (
	logOptions = log.LstdFlags | log.Lmsgprefix | log.Lshortfile
)

var (
	concurrency int
	cli         *client.Client
	pool        *ants.PoolWithFunc
	wg          sync.WaitGroup
)

type pullInfo struct {
	Ctx       context.Context
	Imageref  string
	Waitgroup *sync.WaitGroup
}

func init() {
	flag.IntVar(&concurrency, "parallel", 2, "the number of parallel image pulls to execute at one time")
}

func functionLogger(prefix string) *log.Logger {
	return log.New(os.Stdout, fmt.Sprintf("%s] ", prefix), logOptions)
}

func pullImage(infoIntf interface{}) {
	logger := functionLogger("pullImage")
	info, ok := infoIntf.(*pullInfo)
	if !ok {
		logger.Printf("invalid function input")
		return
	}
	defer info.Waitgroup.Done()
	reader, err := cli.ImagePull(info.Ctx, info.Imageref, types.ImagePullOptions{})
	if err != nil {
		logger.Printf("error pulling image: %s", err.Error())
		return
	}
	defer func() {
		err = reader.Close()
		if err != nil {
			logger.Printf("error closing reader: %s", err.Error())
		}
	}()
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		logger.Printf("error copying response to stdout: %s", err.Error())
		return
	}
}

func main() {
	var err error

	flag.Parse()
	logger := functionLogger("main")
	ctx := context.Background()
	if len(os.Args) <= 1 {
		logger.Fatal("no arguments specified")
	}
	args := os.Args[1:]
	// logger.Printf("os.Args[1:]=%#v", os.Args[1:])
	logger.Printf("initialize pool; concurrency=%d", concurrency)
	pool, err = ants.NewPoolWithFunc(concurrency, pullImage)
	if err != nil {
		logger.Fatalf("error initializing pool: %s", err.Error())
	}
	defer pool.Release()
	logger.Println("initialize client")
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Fatalf("error initializing docker client: %s", err.Error())
	}
	wg = sync.WaitGroup{}
	for _, arg := range args {
		logger.Printf("pulling image %s", arg)
		inf := &pullInfo{
			Ctx:       ctx,
			Imageref:  arg,
			Waitgroup: &wg,
		}
		wg.Add(1)
		err = pool.Invoke(inf)
		if err != nil {
			logger.Fatalf("error pulling image %s: %s", arg, err.Error())
		}
	}
	logger.Println("waiting for tasks to be done")
	wg.Wait()
	logger.Println("done.")
}
