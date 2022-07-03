package internal

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/neflyte/configmap"
)

const (
	TlsCaFile   = "ca.pem"
	TlsCertFile = "cert.pem"
	TlsKeyFile  = "key.pem"
)

var (
	tlsFilenames = [3]string{TlsCaFile, TlsCertFile, TlsKeyFile}
)

type CliContext struct {
	Name     string
	Endpoint *configmap.ConfigMap
	TLSData  *configmap.ConfigMap
}

func NewCliContext() *CliContext {
	endpointMap := configmap.New()
	tlsdataMap := configmap.New()
	return &CliContext{
		Name:     "",
		Endpoint: &endpointMap,
		TLSData:  &tlsdataMap,
	}
}

func ResolveCliContext(contextName string, currentContext bool) (*CliContext, error) {
	logger := FunctionLogger("ResolveCliContext")
	if contextName == "" && !currentContext {
		return nil, errors.New("no context specified")
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		logger.Printf("error looking up user home directory: %s", err.Error())
		return nil, err
	}
	dockerDir := path.Join(userHome, ".docker")
	if currentContext {
		var rawDataBytes []byte
		// Get the name of the current context
		configJsonFile := path.Join(dockerDir, "config.json")
		rawDataBytes, err = ioutil.ReadFile(configJsonFile)
		if err != nil {
			logger.Printf("error reading context config file %s: %s", configJsonFile, err.Error())
			return nil, err
		}
		rawData := configmap.FromJSON(rawDataBytes)
		maybeCurrentContext := rawData.GetStringOrNil("currentContext")
		if maybeCurrentContext == nil {
			return nil, errors.New("current cli context is not defined")
		}
		contextName = *maybeCurrentContext
	}
	if contextName != "" {
		var rawMetaBytes []byte
		resolvedContext := NewCliContext()
		resolvedContext.Name = contextName
		contextRoot := path.Join(dockerDir, "contexts")
		contextMetaDir := path.Join(contextRoot, "meta")
		contextNameHashBytes := sha256.Sum256([]byte(contextName))
		contextNameHash := fmt.Sprintf("%x", contextNameHashBytes)
		contextMetaFile := path.Join(contextMetaDir, contextNameHash, "meta.json")
		rawMetaBytes, err = ioutil.ReadFile(contextMetaFile)
		if err != nil {
			logger.Printf("error reading context metadata file %s: %s", contextMetaFile, err.Error())
			return nil, err
		}
		rawMeta := configmap.FromJSON(rawMetaBytes)
		endpoints := rawMeta.GetConfigMapOrNil("Endpoints")
		if endpoints != nil {
			dockerEndpoint := (*endpoints).GetConfigMapOrNil("docker")
			if dockerEndpoint != nil {
				(*resolvedContext).Endpoint = dockerEndpoint
			}
		}
		contextTLSDir := path.Join(contextRoot, "tls", contextNameHash, "docker")
		for _, tlsFilename := range tlsFilenames {
			contextTLSFilename := path.Join(contextTLSDir, tlsFilename)
			(*resolvedContext.TLSData).Set(tlsFilename, contextTLSFilename)
		}
		return resolvedContext, nil
	}
	return nil, errors.New("no context specified")
}
