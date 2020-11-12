package client

import (
	pb "gitlab.com/wjbbig/grpc-stream/proto"
)

type ClientStub struct {
	MsgId   string
	args    [][]byte
	handler *Handler
}

func newClientStub(handler *Handler, msgId string, input *pb.ContactInput) *ClientStub {
	return &ClientStub{
		MsgId:   msgId,
		args:    input.Args,
		handler: handler,
	}
}

func (c *ClientStub) GetArgs() [][]byte {
	return c.args
}

func (c *ClientStub) GetStringArgs() []string {
	strArgs := make([]string, 0)
	for _, bArg := range c.args {
		strArgs = append(strArgs, string(bArg))
	}
	return strArgs
}

func (c *ClientStub) GetFunctionAndParameters() (string, []string) {
	strArgs := c.GetStringArgs()
	function := ""
	var params []string
	if len(strArgs) >= 1 {
		function = strArgs[0]
		params = strArgs[1:]
	}

	return function, params
}

func (c *ClientStub) InvokeOperation(clientName string, args [][]byte) pb.Response {
	panic("implement me")
}

func (c *ClientStub) GetState(key string) ([]byte, error) {
	return c.handler.handleGetState(key, c.MsgId)
}

func (c *ClientStub) PutState(key string, value []byte) error {
	return c.handler.handlePutState(key, c.MsgId, value)
}

func (c *ClientStub) DelState(key string) error {
	panic("implement me")
}

func (c *ClientStub) GetHistoryForKey(key string) {
	panic("implement me")
}
