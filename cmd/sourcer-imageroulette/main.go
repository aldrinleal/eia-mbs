package main

import (
	"github.com/aldrinleal/eia-mbs/plugininterface"
	"github.com/h2non/filetype"
	"github.com/hashicorp/go-plugin"
	"github.com/joomcode/errorx"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path"
)

type FileData struct {
	Path     string
	MimeType string
}

type SourcerImpl struct {
	count int
	files []*FileData
}

func NewSourcerImpl(dirs []string) (*SourcerImpl, error) {
	fileArray := []*FileData{}

	for _, dir := range dirs {
		dirEntries, err := os.ReadDir(dir)

		if nil != err {
			return nil, errorx.Decorate(err, "reading dir '%s'", dir)
		}

		for _, file := range dirEntries {
			if file.IsDir() {
				continue
			}

			path := path.Join(dir, file.Name())

			data, err := getFileData(path)

			if nil != err {
				log.Warn("while reading for file '%s': %s", file.Name(), err)
				continue
			}

			fileArray = append(fileArray, data)
		}
	}

	log.Infof("picked up '%d' files", len(fileArray))

	return &SourcerImpl{
		count: 0,
		files: fileArray,
	}, nil
}

func getFileData(path string) (*FileData, error) {
	kind, err := filetype.MatchFile(path)

	if nil != err {
		return nil, errorx.Decorate(err, "reading file '%s'", path)
	}

	return &FileData{
		Path:     path,
		MimeType: kind.MIME.Value,
	}, nil
}

func (i *SourcerImpl) GetImage() plugininterface.SourcerReply {
	response := plugininterface.SourcerReply{}

	chosen := i.files[i.count]

	log.Infof("chosen: %+v", chosen)

	data, err := ioutil.ReadFile(chosen.Path)

	if nil != err {
		response.Error = errorx.Decorate(err, "reading path '%s'", chosen.Path)
	} else {
		response.Data = data
		response.Metadata = make(map[string]string)
		response.Metadata["content-type"] = chosen.MimeType
		response.Metadata["name"] = path.Base(chosen.Path)
	}

	i.count = (i.count + 1) % len(i.files)

	return response
}

func main() {
	sourcerImpl, err := NewSourcerImpl(os.Args[1:])

	if nil != err {
		log.Fatalf("creating sourcer: %s", err)
	}

	pluginMap := map[string]plugin.Plugin{
		"sourcer": &plugininterface.SourcerPlugin{Impl: sourcerImpl},
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugininterface.HandshakeConfig,
		Plugins:         pluginMap,
	})

}
