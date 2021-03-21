package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/neflyte/configmap"
	"github.com/neflyte/uiprogress"
	"github.com/panjf2000/ants/v2"
)

const (
	logOptions        = log.LstdFlags | log.Lmsgprefix | log.Lshortfile
	oneHundred        = 100
	oneHundredPercent = 100.00
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
	flag.IntVar(&concurrency, "parallel", 2, "the number of parallel image pull requests to execute at one time")
}

func functionLogger(prefix string) *log.Logger {
	return log.New(os.Stdout, fmt.Sprintf("%s] ", prefix), logOptions)
}

func pullImage(infoIntf interface{}) {
	var currentJN, totalJN json.Number
	var current, total float64
	var barPrefix string
	var rawMap map[string]interface{}

	logger := functionLogger("pullImage")
	info, ok := infoIntf.(*pullInfo)
	if !ok {
		logger.Printf("invalid function input")
		return
	}
	defer info.Waitgroup.Done()
	bar := uiprogress.AddBar(oneHundred).
		AppendCompleted().
		PrependFunc(func(b *uiprogress.Bar) string {
			return fmt.Sprintf("%s: %s", info.Imageref, barPrefix)
		}).
		NoProgressBar()
	defer func() {
		err := bar.Set(oneHundred)
		if err != nil {
			logger.Printf("error setting bar: %s", err.Error())
		}
	}()
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
	dec := json.NewDecoder(reader)
	dec.UseNumber()
	for dec.More() {
		rawMap = make(map[string]interface{})
		err = dec.Decode(&rawMap)
		if err != nil {
			logger.Printf("error decoding: %s", err.Error())
			return
		}
		cmap := configmap.Configmap(rawMap)
		barPrefix = strings.TrimPrefix(cmap.GetString("status"), "Status: ")
		progressDetailPtr := cmap.GetConfigMapOrNil("progressDetail")
		if progressDetailPtr != nil {
			progressDetail := *progressDetailPtr
			currentIntf := progressDetail.GetOrNil("current")
			if currentIntf != nil {
				currentJN, ok = currentIntf.(json.Number)
				if !ok {
					logger.Printf("error casting current to json.Number; value: %#v", currentIntf)
					return
				}
				current, err = currentJN.Float64()
				if err != nil {
					logger.Printf("error getting float64 value from currentJN: %s", err.Error())
					return
				}
			}
			totalIntf := progressDetail.GetOrNil("total")
			if totalIntf != nil {
				totalJN, ok = totalIntf.(json.Number)
				if !ok {
					logger.Printf("error casting total to json.Number; value: %#v", totalIntf)
					return
				}
				total, err = totalJN.Float64()
				if err != nil {
					logger.Printf("error getting float64 value from totalJN: %s", err.Error())
					return
				}
			}
		}
		if total > 0 {
			percent := int(math.Round((current / total) * oneHundredPercent))
			err = bar.Set(percent)
			if err != nil {
				logger.Printf("error setting bar value: %s", err.Error())
			}
		}
	}
}

func main() {
	var err error

	flag.Parse()
	logger := functionLogger("main")
	ctx := context.Background()
	if len(flag.Args()) <= 1 {
		logger.Fatal("no arguments specified")
	}
	pool, err = ants.NewPoolWithFunc(concurrency, pullImage)
	if err != nil {
		logger.Fatalf("error initializing pool: %s", err.Error())
	}
	defer pool.Release()
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Fatalf("error initializing docker client: %s", err.Error())
	}
	wg = sync.WaitGroup{}
	uiprogress.Start()
	for _, arg := range flag.Args() {
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
	wg.Wait()
	uiprogress.Stop()
	fmt.Println("done.")
}
