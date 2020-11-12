package client

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	pb "gitlab.com/wjbbig/grpc-stream/proto"
	"log"
	"sync"
)

// client的状态信息
type state string

const (
	created     state = "created"     // 启动
	established state = "established" // 已连接服务端
	ready       state = "ready"       // 准备开始信息交互
)

type ServerContactStream interface {
	Send(*pb.ContactMessage) error
	Recv() (*pb.ContactMessage, error)
}

type ClientStream interface {
	ServerContactStream
	CloseSend() error
}

type Handler struct {
	// 用于防止发送消息时的并发调用
	serialLock sync.Mutex
	// 用于与服务端交互
	chatStream ServerContactStream
	// 定义了client端的所有动作，使用者需要实现这个接口
	operation Operation
	// client的状态信息
	state state

	responseChannelsMutex sync.Mutex
	responseChannels      map[string]chan pb.ContactMessage
}

func newHandler(chatSteam ServerContactStream, op Operation) *Handler {
	return &Handler{
		chatStream:       chatSteam,
		operation:        op,
		state:            created,
		responseChannels: map[string]chan pb.ContactMessage{},
	}
}

// shortMsgId 只获取消息ID的前8位，仅用于缩短错误信息的长度
func shortMsgId(msgId string) string {
	if len(msgId) < 8 {
		return msgId
	}
	return msgId[:8]
}

// createResponseChannel 创建一个用于接收特定回复的通道
func (h *Handler) createResponseChannel(msgId string) (<-chan pb.ContactMessage, error) {
	h.responseChannelsMutex.Lock()
	defer h.responseChannelsMutex.Unlock()
	// 判断是否已经初始化responseChannels
	if h.responseChannels == nil {
		return nil, fmt.Errorf("[%s] cannot create response channel", shortMsgId(msgId))
	}
	// 判断指定ID的通道是否存在
	if h.responseChannels[msgId] != nil {
		return nil, fmt.Errorf("[%s] channel exists", shortMsgId(msgId))
	}
	responseChan := make(chan pb.ContactMessage)
	h.responseChannels[msgId] = responseChan
	return responseChan, nil
}

// deleteResponseChannel 删除指定msgId的回复通道
func (h *Handler) deleteResponseChannel(msgId string) {
	h.responseChannelsMutex.Lock()
	defer h.responseChannelsMutex.Unlock()

	if h.responseChannels != nil {
		delete(h.responseChannels, msgId)
	}
}

// sendAndReceive 发送并接收消息
func (h *Handler) sendAndReceive(msg *pb.ContactMessage, responseChan <-chan pb.ContactMessage) (
	pb.ContactMessage, error) {
	err := h.serialSend(msg)
	if err != nil {
		return pb.ContactMessage{}, nil
	}

	outMsg := <-responseChan
	return outMsg, nil
}

// callServerWithMsg 调用grpc发送并接受消息
func (h *Handler) callServerWithMsg(msg *pb.ContactMessage, msgId string) (pb.ContactMessage, error) {
	//创建
	responseChan, err := h.createResponseChannel(msgId)
	if err != nil {
		return pb.ContactMessage{}, nil
	}
	defer h.deleteResponseChannel(msgId)
	return h.sendAndReceive(msg, responseChan)
}

// 用于进行操作的通用方法，对消息进行处理，并生成回复消息
type stubHandlerFunc func(message *pb.ContactMessage) (*pb.ContactMessage, error)

// handleStubInteraction 根据传入的handler来处理消息
func (h *Handler) handleStubInteraction(handler stubHandlerFunc, msg *pb.ContactMessage, errc chan<- error) {
	// 处理消息
	resp, err := handler(msg)
	if err != nil {
		resp = &pb.ContactMessage{Type: pb.ContactMessage_Type_ERROR, Payload: []byte(err.Error()), MsgId: msg.MsgId}
	}
	h.serialSendAsync(resp, errc)
}

// handleInit 处理初始化操作
func (h *Handler) handleInit(msg *pb.ContactMessage) (*pb.ContactMessage, error) {
	input := new(pb.ContactInput)
	err := proto.Unmarshal(msg.Payload, input)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to unmarshal input.")
	}

	stub := newClientStub(h, msg.MsgId, input)
	resp := h.operation.Init(stub)
	if resp.Status >= ERROR {
		return &pb.ContactMessage{Type: pb.ContactMessage_Type_ERROR, Payload: []byte(resp.Message), MsgId: msg.MsgId}, nil
	}

	resBytes, err := proto.Marshal(&resp)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to marshal response.")
	}

	return &pb.ContactMessage{Type: pb.ContactMessage_Type_COMPLETED, Payload: resBytes, MsgId: msg.MsgId}, nil
}

// handleResponse 处理来自server端的回复，将回复信息放入相应的通道中
func (h *Handler) handleResponse(msg *pb.ContactMessage) error {
	h.responseChannelsMutex.Lock()
	defer h.responseChannelsMutex.Unlock()

	if h.responseChannels == nil {
		return fmt.Errorf("[%s] Cannot send message response channel", shortMsgId(msg.MsgId))
	}

	responseChan := h.responseChannels[msg.MsgId]
	if responseChan == nil {
		return fmt.Errorf("[%s] responseChannel does not exist", shortMsgId(msg.MsgId))
	}
	responseChan <- *msg
	return nil
}

func (h *Handler) handleTransaction(msg *pb.ContactMessage) (*pb.ContactMessage, error) {
	input := new(pb.ContactInput)
	err := proto.Unmarshal(msg.Payload, input)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to unmarshal input.")
	}
	stub := newClientStub(h, msg.MsgId, input)
	resp := h.operation.Invoke(stub)
	resBytes, err := proto.Marshal(&resp)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to marshal response.")
	}

	return &pb.ContactMessage{Type: pb.ContactMessage_Type_COMPLETED, Payload: resBytes, MsgId: msg.MsgId}, nil
}

// handleGetState 向server发送获取持久化数据的请求
func (h *Handler) handleGetState(key, msgId string) ([]byte, error) {
	payload := marshalOrPanic(&pb.GetState{Key: key})

	msg := &pb.ContactMessage{Type: pb.ContactMessage_Type_GET_STATE, Payload: payload, MsgId: msgId}
	respMsg, err := h.callServerWithMsg(msg, msgId)
	if err != nil {
		return nil, fmt.Errorf("[%s] error sending %s: %s", shortMsgId(msgId), pb.ContactMessage_Type_GET_STATE, err)
	}

	if respMsg.Type == pb.ContactMessage_Type_RESPONSE {
		return respMsg.Payload, nil
	}
	if respMsg.Type == pb.ContactMessage_Type_ERROR {
		return nil, fmt.Errorf("%s", respMsg.Payload[:])
	}
	// 返回了不对劲的Type
	return nil, fmt.Errorf("[%s] incorrect message %s received. Expecting %s or %s", shortMsgId(respMsg.MsgId),
		respMsg.Type, pb.ContactMessage_Type_ERROR, pb.ContactMessage_Type_GET_STATE)
}

// handlePutState 向server发送持久化数据请求
func (h *Handler) handlePutState(key, msgId string, value []byte) error {
	payload := marshalOrPanic(&pb.PutState{Key: key, Value: value})

	msg := &pb.ContactMessage{Type: pb.ContactMessage_Type_PUT_STATE, Payload: payload, MsgId: msgId}
	respMsg, err := h.callServerWithMsg(msg, msgId)
	if err != nil {
		return errors.WithMessagef(err, "[%s] failed to send %s", msg.MsgId, pb.ContactMessage_Type_PUT_STATE)
	}

	if respMsg.Type == pb.ContactMessage_Type_RESPONSE {
		return nil
	}
	if respMsg.Type == pb.ContactMessage_Type_ERROR {
		return fmt.Errorf("%s", respMsg.Payload[:])
	}

	return fmt.Errorf("[%s] incorrect message %s received. Expecting %s or %s", shortMsgId(respMsg.MsgId),
		respMsg.Type, pb.ContactMessage_Type_ERROR, pb.ContactMessage_Type_RESPONSE)
}

func (h *Handler) handleDelState(key, msgId string) error {
	panic("implement me")
}

func (h *Handler) handleGetHistoryForKey(key, msgId string) {
	panic("implement me")
}

// serialSend 调用gRPC的send方法给server端发消息
func (h *Handler) serialSend(msg *pb.ContactMessage) error {
	h.serialLock.Lock()
	defer h.serialLock.Unlock()
	return h.chatStream.Send(msg)
}

// serialSendAsync 使用goroutine异步发送消息，并使用chan接受所有的错误
func (h *Handler) serialSendAsync(msg *pb.ContactMessage, errc chan<- error) {
	go func() {
		errc <- h.serialSend(msg)
	}()
}

// handleCreated 在handler的状态为“created”时，处理server的消息
func (h *Handler) handleCreated(msg *pb.ContactMessage, errc chan error) error {
	if msg.Type != pb.ContactMessage_Type_REGISTERED {
		return fmt.Errorf("handler cannot handle message (%s) while in state: %s", msg.Type, h.state)
	}
	log.Println("client changed state to established")
	h.state = established
	return nil
}

// handleEstablished 在handler的状态为“established”时，处理server的消息
func (h *Handler) handleEstablished(msg *pb.ContactMessage, errc chan error) error {
	if msg.Type != pb.ContactMessage_Type_READY {
		return fmt.Errorf("handler cannot handle message (%s) while in state: %s", msg.Type, h.state)
	}
	log.Println("client changed state to ready")
	h.state = ready
	return nil
}

func (h *Handler) handleReady(msg *pb.ContactMessage, errc chan error) error {
	switch msg.Type {
	case pb.ContactMessage_Type_RESPONSE, pb.ContactMessage_Type_ERROR:
		if err := h.handleResponse(msg); err != nil {
			return err
		}
		return nil
	case pb.ContactMessage_Type_INIT:
		go h.handleStubInteraction(h.handleInit, msg, errc)
		return nil
	case pb.ContactMessage_Type_INVOKE:
		go h.handleStubInteraction(h.handleTransaction, msg, errc)
		return nil
	default:
		return fmt.Errorf("handler cannot handle message (%s) while in state: %s", msg.Type, h.state)
	}
}

// handleMessage 在循环中不断的处理来自server端的消息
func (h *Handler) handleMessage(msg *pb.ContactMessage, errc chan error) error {
	log.Printf("[%s] Client side handling contactMessage of type: %s in state %s", shortMsgId(msg.MsgId), msg.Type, h.state)
	if msg.Type == pb.ContactMessage_Type_KEEPALIVE {
		h.serialSendAsync(msg, errc)
		return nil
	}
	var err error

	// 更新状态来处理消息
	switch h.state {
	case created:
		err = h.handleCreated(msg, errc)
	case established:
		err = h.handleEstablished(msg, errc)
	case ready:
		err = h.handleReady(msg, errc)
	default:
		panic(fmt.Sprintf("invalid handler state: %s", h.state))
	}

	if err != nil {
		payload := []byte(err.Error())
		errorMsg := &pb.ContactMessage{Type: pb.ContactMessage_Type_ERROR, Payload: payload, MsgId: msg.MsgId}
		h.serialSend(errorMsg)
		return err
	}

	return nil
}

func marshalOrPanic(msg proto.Message) []byte {
	bytes, err := proto.Marshal(msg)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal message: %s", err))
	}
	return bytes
}
