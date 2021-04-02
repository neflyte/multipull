package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"multipull/internal"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/neflyte/configmap"
	"github.com/neflyte/uiprogress"
	"github.com/panjf2000/ants/v2"
)

const (
	oneHundred        = 100
	oneHundredPercent = 100.00
)

var (
	AppVersion = "dev" // AppVersion is the application version string

	cliContextName    string
	cliCurrentContext bool
	concurrency       int

	cli  *client.Client
	pool *ants.PoolWithFunc
	wg   sync.WaitGroup
)

type pullInfo struct {
	Ctx       context.Context
	Imageref  string
	Waitgroup *sync.WaitGroup
}

func pullImage(infoIntf interface{}) {
	var current, total float64
	var barPrefix string
	var cmap configmap.Configmap

	logger := internal.FunctionLogger("pullImage")
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
		barPrefix = fmt.Sprintf("error pulling image: %s", err.Error())
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
		cmap = make(configmap.Configmap)
		err = dec.Decode(&cmap)
		if err != nil {
			logger.Printf("error decoding: %s", err.Error())
			continue
		}
		barPrefix = strings.TrimPrefix(cmap.GetString("status"), "Status: ")
		progressDetailPtr := cmap.GetConfigMapOrNil("progressDetail")
		if progressDetailPtr != nil {
			currentIntf := (*progressDetailPtr).GetJSONNumberAsFloat64OrNil("current")
			if currentIntf == nil {
				current = 0
			} else {
				current = *currentIntf
			}
			totalIntf := (*progressDetailPtr).GetJSONNumberAsFloat64OrNil("total")
			if totalIntf == nil {
				total = 0
			} else {
				total = *totalIntf
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
	var cliContext *internal.CliContext

	flag.IntVar(&concurrency, "parallel", 2, "the number of parallel image pull requests to execute at one time")
	flag.StringVar(&cliContextName, "context", "", "the docker cli context to use (optional)")
	flag.BoolVar(&cliCurrentContext, "current-context", false, "use the current docker cli context; supercedes -context (optional)")

	fmt.Printf("multipull v%s\n--\n", AppVersion)
	flag.Parse()
	logger := internal.FunctionLogger("main")
	ctx := context.Background()
	if len(flag.Args()) < 1 {
		logger.Fatal("no arguments specified")
	}
	pool, err = ants.NewPoolWithFunc(concurrency, pullImage)
	if err != nil {
		logger.Fatalf("error initializing pool: %s", err.Error())
	}
	defer pool.Release()
	// Was a docker cli context supplied?
	if cliContextName != "" || cliCurrentContext {
		cliContext, err = internal.ResolveCliContext(cliContextName, cliCurrentContext)
		if err != nil {
			logger.Fatalf("error resolving cli context: %s", err.Error())
		}
		contextHost := (*cliContext.Endpoint).GetStringOrNil("Host")
		if contextHost == nil {
			logger.Fatalf("host not defined for context %s", cliContext.Name)
			return
		}
		cliOpts := []client.Opt{
			client.WithAPIVersionNegotiation(),
			client.WithHost(*contextHost),
		}
		if (*cliContext.TLSData).Has(internal.TlsCaFile) &&
			(*cliContext.TLSData).Has(internal.TlsCertFile) &&
			(*cliContext.TLSData).Has(internal.TlsKeyFile) {
			cliOpts = append(
				cliOpts,
				client.WithTLSClientConfig(
					(*cliContext.TLSData).GetString(internal.TlsCaFile),
					(*cliContext.TLSData).GetString(internal.TlsCertFile),
					(*cliContext.TLSData).GetString(internal.TlsKeyFile),
				),
			)
		}
		cli, err = client.NewClientWithOpts(cliOpts...)
	} else {
		cli, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
	}
	if err != nil {
		logger.Fatalf("error initializing docker client: %s", err.Error())
	}
	defer func() {
		err = cli.Close()
		if err != nil {
			logger.Printf("error closing client: %s", err.Error())
		}
	}()
	wg = sync.WaitGroup{}
	uiprogress.Start()
	for _, arg := range flag.Args() {
		wg.Add(1)
		err = pool.Invoke(&pullInfo{
			Ctx:       ctx,
			Imageref:  arg,
			Waitgroup: &wg,
		})
		if err != nil {
			logger.Printf("error pulling image %s: %s", arg, err.Error())
		}
	}
	wg.Wait()
	uiprogress.Stop()
	fmt.Println("done.")
}
