package plugininterface

import (
	"github.com/hashicorp/go-plugin"
	"net/rpc"
)

type SourcerReply struct {
	Data     []byte
	Metadata map[string]string
	Error    error
}

type Sourcer interface {
	GetImage() SourcerReply
}

type SourcerRPC struct {
	client *rpc.Client
}

func (s *SourcerRPC) GetImage() SourcerReply {
	var resp SourcerReply

	err := s.client.Call("Plugin.GetImage", new(interface{}), &resp)

	if nil != err {
		resp.Error = err
	}

	return resp
}

type SourcerRPCServer struct {
	Impl Sourcer
}

func (s *SourcerRPCServer) GetImage(args interface{}, reply *SourcerReply) error {
	*reply = s.Impl.GetImage()

	return nil
}

type SourcerPlugin struct {
	Impl Sourcer
}

func (p *SourcerPlugin) Server(broker *plugin.MuxBroker) (interface{}, error) {
	return &SourcerRPCServer{Impl: p.Impl}, nil
}

func (SourcerPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &SourcerRPC{client: c}, nil
}

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "BASIC_PLUGIN",
	MagicCookieValue: "hello",
}
