package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	config "github.com/aldrinleal/eia-mbs/config"
	"github.com/aldrinleal/eia-mbs/plugininterface"
	"github.com/docopt/docopt-go"
	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/go-plugin"
	"github.com/joomcode/errorx"
	"github.com/robertkrimen/otto"
	_ "github.com/robertkrimen/otto/underscore"
	log "github.com/sirupsen/logrus"
	"github.com/tbrandon/mbserver"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"strings"
	"time"
)

var pluginMap = map[string]plugin.Plugin{
	"sourcer": &plugininterface.SourcerPlugin{},
}

const DOC = `mbs.

Usage:
	mbs CONFIGFILE
`

func loadConfigFile() (*config.Config, error) {
	opts, err := docopt.Parse(DOC, nil, true, "0.0.1", true)

	if nil != err {
		return nil, err
	}

	configPath := opts["CONFIGFILE"].(string)

	configFile, err := os.OpenFile(configPath, os.O_RDONLY, os.FileMode(0))

	if nil != err {
		return nil, errorx.Decorate(err, "opening file '%s'", configPath)
	}

	defer configFile.Close()

	config := &config.Config{}

	err = yaml.NewDecoder(configFile).Decode(config)

	if nil != err {
		return nil, errorx.Decorate(err, "parsing config file '%s'", configPath)
	}

	config.ServiceUrl = strings.TrimSpace(config.ServiceUrl)
	config.SourcerCmd = strings.TrimSpace(config.SourcerCmd)
	config.ListenAddr = strings.TrimSpace(config.ListenAddr)

	return config, nil
}

func main() {
	config, err := loadConfigFile()

	if nil != err {
		log.Fatalf("Parsing config: ", err)
	}

	log.Debugf("config: '%+v'", config)

	vm := otto.New()

	_, err = vm.Run(config.GraderFunc)

	if nil != err {
		log.Fatalf("Grading function: %s", err)
	}

	server := mbserver.NewServer()

	pluginCommand := exec.Command(os.Getenv("SHELL"), "-c", config.SourcerCmd)

	pluginCommand.Env = os.Environ()

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: plugininterface.HandshakeConfig,
		Plugins:         pluginMap,
		Cmd:             pluginCommand,
	})

	defer client.Kill()

	rpcClient, err := client.Client()

	if nil != err {
		log.Fatalf("procuring sourcer: %s", err)
	}

	raw, err := rpcClient.Dispense("sourcer")

	if nil != err {
		log.Fatalf("dispensing: %s", err)
	}

	sourcer := raw.(plugininterface.Sourcer)

	log.Infof("obtained sourcer: %+v", sourcer)

	restyClient := resty.New()

	registerAddressAndValue := func(frame mbserver.Framer) (int, uint16) {
		data := frame.GetData()
		register := int(binary.BigEndian.Uint16(data[0:2]))
		value := binary.BigEndian.Uint16(data[2:4])
		return register, value
	}

	server.RegisterFunctionHandler(6, func(s *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
		register, value := registerAddressAndValue(frame)

		log.Infof("register: %d value: %d", register, value)

		go func() {
			s.HoldingRegisters[register] = 0
			s.HoldingRegisters[register+1] = 0
			s.HoldingRegisters[register+2] = 0

			log.Infof("calling sourcer")

			s.HoldingRegisters[register] = 1

			sourcerReply := sourcer.GetImage()

			if nil != sourcerReply.Error {
				log.Warnf("Failure when sourcing: %s", sourcerReply.Error)

				s.HoldingRegisters[register+1] = 1

				return
			}

			log.Infof("obtained image (metadata: %+v)", sourcerReply.Metadata)

			resp, err := restyClient.R().
				SetMultipartFields(
					&resty.MultipartField{
						Param:       "file",
						FileName:    sourcerReply.Metadata["name"],
						ContentType: sourcerReply.Metadata["content-type"],
						Reader:      bytes.NewBuffer(sourcerReply.Data),
					},
				).
				SetContentLength(true).
				Post(config.ServiceUrl)

			s.HoldingRegisters[register] = 2

			if nil != err {
				s.HoldingRegisters[register+1] = 2
				return
			}

			log.Infof("called service. err: %s resp: %+v", err, resp)

			s.HoldingRegisters[register] = 3
			s.HoldingRegisters[register+1] = 0

			expression := fmt.Sprintf("grader(%s)", resp.String())

			mappedRegisterValue, err := vm.Run(expression)

			if nil != err {
				log.Warnf("Oops: %s", errorx.Decorate(err, "grading response"))

				s.HoldingRegisters[register+1] = 4

				return
			}

			s.HoldingRegisters[register] = 4
			s.HoldingRegisters[register+1] = 0

			registerValueAsInteger, err := mappedRegisterValue.ToInteger()

			if nil != err {
				log.Warnf("Oops: %s", errorx.Decorate(err, "grading response"))

				s.HoldingRegisters[register+1] = 5

				return
			}

			log.Infof("registerValueAsInteger: %d", registerValueAsInteger)

			s.HoldingRegisters[register+2] = uint16(registerValueAsInteger)

			s.HoldingRegisters[register] = 5
		}()

		return frame.GetData()[0:4], &mbserver.Success
	})

	err = server.ListenTCP(config.ListenAddr)

	if nil != err {
		log.Fatalf("listening to server: %s", errorx.Decorate(err, "listening to server"))
	}

	defer server.Close()

	for {
		time.Sleep(1 * time.Second)
	}

}
