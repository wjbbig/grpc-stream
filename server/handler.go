package server

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	pb "gitlab.com/wjbbig/grpc-stream/proto"
	"io"
	"log"
	"sync"
	"time"
)

type ClientStream interface {
	Send(*pb.ContactMessage) error
	Recv() (*pb.ContactMessage, error)
}

// 注册与client相应的handler
type Registry interface {
	Register(*Handler) error
	Deregister(string) error
}

// 用于保存正在执行的信息，防止重复调用
type MessageRegistry interface {
	Add(msgId string) bool
	Remove(msgId string)
}

// 用于描述client端的状态
type state string

const (
	created     state = "created"
	established state = "established"
	ready       state = "ready"
)

type Handler struct {
	// 心跳消息的间隔
	KeepAlive time.Duration
	// client注册机，一个handler与一个client对应
	Registry Registry
	// 追踪client状态
	state state
	// 保存client端数据
	stateStore *bolt.DB
	// client的名字
	clientName string
	// 用于阻止并发发送消息的锁
	serialLock sync.Mutex
	// 与client进行grpc沟通的流
	chatStream ClientStream
	// 存放异步发送消息时发生的错误
	errChan chan error
	// 用于streamDoneChan的锁
	mutex sync.Mutex
	// 追踪哪些消息正在被执行
	activeMsgs MessageRegistry
	// 接收不同msg的回复消息
	responseNotifier *ResponseNotifier
	// 程序退出，结束所有的消息处理
	streamDoneChan chan struct{}
}

func shortMsgId(msgId string) string {
	if len(msgId) < 8 {
		return msgId
	}
	return msgId[0:8]
}

// handleMessage 在ProcessStream方法中被调用，用于区分消息
func (h *Handler) handleMessage(msg *pb.ContactMessage) error {
	log.Printf("[%s] Server side handling contactMessage of type: %s in state %s", shortMsgId(msg.MsgId), msg.Type, h.state)
	if msg.Type == pb.ContactMessage_Type_KEEPALIVE {
		return nil
	}

	switch h.state {
	case created:
		return h.handleMessageCreatedState(msg)
	case ready:
		return h.handleMessageReadyState(msg)
	default:
		return errors.Errorf("Handle message: invalid state %s for transaction %s", h.state, msg.MsgId)
	}
}

// handleMessageCreatedState 处理client注册信息
func (h *Handler) handleMessageCreatedState(msg *pb.ContactMessage) error {
	switch msg.Type {
	case pb.ContactMessage_Type_REGISTER:
		h.handleRegister(msg)
	default:
		return errors.Errorf("[%s] Handler cannot handle message (%s) while in created state", msg.MsgId, msg.Type)
	}
	return nil
}

// handleRegister 将client注册到server
func (h *Handler) handleRegister(msg *pb.ContactMessage) {
	log.Printf("Receive %s in state %s", msg.Type, h.state)
	clientName := &pb.ClientName{}
	err := proto.Unmarshal(msg.Payload, clientName)
	if err != nil {
		log.Printf("Error in received %s, could NOT unmarshal registration info: %s", pb.ContactMessage_Type_REGISTER, err)
		return
	}

	// client传过来的名字为空，取消注册
	if clientName.Name == "" {
		h.notifyRegistry(errors.New("error in handling register client, clientName is empty"))
		return
	}
	h.clientName = clientName.Name
	// 注册出错，取消注册
	err = h.Registry.Register(h)
	if err != nil {
		h.notifyRegistry(err)
		return
	}
	// 发送REGISTERED消息失败，取消注册
	if err := h.serialSend(&pb.ContactMessage{Type: pb.ContactMessage_Type_REGISTERED}); err != nil {
		log.Printf("error sending %s: %s", pb.ContactMessage_Type_REGISTERED, err)
		h.notifyRegistry(err)
		return
	}
	h.state = established
	log.Printf("changed state to established for %s\n", h.clientName)
	h.notifyRegistry(nil)
}

func (h *Handler) handleMessageReadyState(msg *pb.ContactMessage) error {
	switch msg.Type {
	case pb.ContactMessage_Type_COMPLETED, pb.ContactMessage_Type_ERROR:
		h.notify(msg)
	case pb.ContactMessage_Type_PUT_STATE:
		go h.handleTransaction(msg, h.handlePutState)
	case pb.ContactMessage_Type_GET_STATE:
		go h.handleTransaction(msg, h.handleGetState)
	default:
		return fmt.Errorf("[%s] Handler cannot handle message (%s) while in ready state", msg.MsgId, msg.Type)
	}

	return nil
}

type handleFunc func(message *pb.ContactMessage) (*pb.ContactMessage, error)

// handleTransaction 根据handleFunc来处理消息，并返回给client
func (h *Handler) handleTransaction(msg *pb.ContactMessage, handleFunc handleFunc) {
	log.Printf("[%s] handling %s from client", shortMsgId(msg.MsgId), msg.Type.String())
	// 判断是否有相同ID的消息在被处理
	if !h.registerMsgId(msg) {
		return
	}
	resp, err := handleFunc(msg)
	if err != nil {
		err = errors.Wrapf(err, "%s failed: msgId: %s", msg.Type, msg.MsgId)
		log.Printf("[%s] Failed to handle %s. error: %+v", shortMsgId(msg.MsgId), msg.Type, err)
		resp = &pb.ContactMessage{Type: pb.ContactMessage_Type_ERROR, Payload: []byte(err.Error()), MsgId: msg.MsgId}
	}

	log.Printf("[%s] completed %s. Sending %s", shortMsgId(msg.MsgId), msg.Type, resp.Type)
	h.activeMsgs.Remove(msg.MsgId)
	h.serialSendAsync(resp)
}

func (h *Handler) handlePutState(msg *pb.ContactMessage) (*pb.ContactMessage, error) {
	putState := &pb.PutState{}
	err := proto.Unmarshal(msg.Payload, putState)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}
	err = h.stateStore.Update(func(tx *bolt.Tx) error {
		var err error
		bucket := tx.Bucket([]byte(h.clientName))
		if bucket == nil {
			bucket, err = tx.CreateBucket([]byte(h.clientName))
			if err != nil {
				return err
			}
		}

		err = bucket.Put([]byte(putState.Key), putState.Value)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "put state failed")
	}

	return &pb.ContactMessage{Type: pb.ContactMessage_Type_RESPONSE, MsgId: msg.MsgId}, nil
}

func (h *Handler) handleGetState(msg *pb.ContactMessage) (*pb.ContactMessage, error) {
	getState := &pb.GetState{}
	err := proto.Unmarshal(msg.Payload, getState)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}
	var res []byte
	err = h.stateStore.Update(func(tx *bolt.Tx) error {
		var err error
		bucket := tx.Bucket([]byte(h.clientName))
		if bucket == nil {
			bucket, err = tx.CreateBucket([]byte(h.clientName))
			if err != nil {
				return err
			}
		}

		res = bucket.Get([]byte(getState.Key))
		return nil
	})
	log.Printf("-----------------%s\n", string(res))
	if err != nil {
		return nil, errors.Wrap(err, "get state failed")
	}

	return &pb.ContactMessage{Type: pb.ContactMessage_Type_RESPONSE, Payload: res, MsgId: msg.MsgId}, nil
}

// serialSend 向client端发送消息
func (h *Handler) serialSend(msg *pb.ContactMessage) error {
	h.serialLock.Lock()
	defer h.serialLock.Unlock()

	if err := h.chatStream.Send(msg); err != nil {
		err = errors.WithMessagef(err, "[%s] error sending %s", shortMsgId(msg.MsgId), msg.Type)
		log.Printf("%+v", err)
		return err
	}

	return nil
}

// serialSendAsync 异步发送消息给client，将错误发送到管道中
func (h *Handler) serialSendAsync(msg *pb.ContactMessage) {
	go func() {
		if err := h.serialSend(msg); err != nil {
			// 这里有错直接就返回一个错误ContactMessage
			resp := &pb.ContactMessage{
				Type:    pb.ContactMessage_Type_ERROR,
				Payload: []byte(err.Error()),
				MsgId:   msg.MsgId,
			}
			h.notify(resp)
			// 发送错误信息，关闭连接
			h.errChan <- err
		}
	}()
}

func (h *Handler) deregister() {
	h.Registry.Deregister(h.clientName)
}

func (h *Handler) streamDone() <-chan struct{} {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return h.streamDoneChan
}

func (h *Handler) ProcessStream(stream ClientStream) error {
	defer h.deregister()

	h.mutex.Lock()
	h.streamDoneChan = make(chan struct{})
	h.mutex.Unlock()
	defer close(h.streamDoneChan)

	h.chatStream = stream
	h.errChan = make(chan error, 1)

	var keepaliveCh <-chan time.Time
	if h.KeepAlive != 0 {
		ticker := time.NewTicker(h.KeepAlive)
		defer ticker.Stop()
		keepaliveCh = ticker.C
	}

	// 获取gRPC的消息
	type recvMsg struct {
		msg *pb.ContactMessage
		err error
	}
	msgAvail := make(chan *recvMsg, 1)

	receiveMessage := func() {
		in, err := h.chatStream.Recv()
		msgAvail <- &recvMsg{in, err}
	}

	go receiveMessage()
	for {
		select {
		case rmsg := <-msgAvail:
			switch {
			case rmsg.err == io.EOF:
				log.Printf("receive EOF, ending chaincode support stream:%s\n", rmsg.err)
				return rmsg.err
			case rmsg.err != nil:
				err := errors.Wrap(rmsg.err, "receive from client stream failed")
				log.Printf("%+v\n", err)
				return err
			case rmsg.msg == nil:
				err := errors.New("received nil message, ending client stream")
				log.Printf("%+v\n", err)
				return err
			default:
				err := h.handleMessage(rmsg.msg)
				if err != nil {
					err = errors.WithMessage(err, "error handling message, ending stream")
					log.Printf("[%s] %+v", shortMsgId(rmsg.msg.MsgId), err)
					return err
				}

				go receiveMessage()
			}
		case sendErr := <-h.errChan:
			err := errors.Wrap(sendErr, "received error while sending message, ending client client stream")
			log.Printf("%s", err)
			return err
		case <-keepaliveCh:
			h.serialSendAsync(&pb.ContactMessage{Type: pb.ContactMessage_Type_KEEPALIVE})
			continue
		}
	}
}

// sendReady 发送READY状态给client
func (h *Handler) sendReady() error {
	log.Printf("sending READY for client %s\n", h.clientName)
	if err := h.serialSend(&pb.ContactMessage{Type: pb.ContactMessage_Type_READY}); err != nil {
		log.Printf("error sending READY (%s) for client %s\n", err, h.clientName)
		return err
	}

	h.state = ready
	log.Printf("changed to state ready for client %s\n", h.clientName)
	return nil
}

// notify 将client回复消息放入到指定的notifier管道中
func (h *Handler) notify(msg *pb.ContactMessage) {
	notifier, ok := h.responseNotifier.Notify(msg.MsgId)
	if !ok {
		log.Printf("notifier msgId:%s does not exist for handling message %s\n", msg.MsgId, msg.Type)
		return
	}

	log.Printf("[%s] notifying msgId:%s\n", shortMsgId(msg.MsgId), msg.MsgId)
	notifier <- msg
}

// notifyRegistry 注册之后的后续工作
func (h *Handler) notifyRegistry(err error) {
	if err == nil {
		err = h.sendReady()
	}

	//if err != nil {
	//	h.Registry.Failed(h.clientName, err)
	//	log.Printf("failed to start %s -- %s", h.clientName, err)
	//	return
	//}
	//
	//h.Registry.Ready(h.clientName)
}

// registerMsgId 将消息ID注册，防止重复处理消息
func (h *Handler) registerMsgId(msg *pb.ContactMessage) bool {
	if h.activeMsgs.Add(msg.MsgId) {
		return true
	}
	log.Printf("another message with msgId %s is processing\n", msg.MsgId)
	return false
}

func (h *Handler) Execute(msg *pb.ContactMessage, timeout time.Duration) (*pb.ContactMessage, error) {
	log.Println("Entry")
	defer log.Println("Exit")
	var err error
	respCh, ok := h.responseNotifier.Add(msg.MsgId)
	if !ok {
		return nil, errors.New("")
	}
	defer h.responseNotifier.Delete(msg.MsgId)

	h.serialSendAsync(msg)

	var cresp *pb.ContactMessage
	select {
	case cresp = <-respCh:

	case <-time.After(timeout):
		err = errors.New("timeout expired while executing message")
	case <-h.streamDone():
		err = errors.New("client stream terminated")
	}

	return cresp, err
}
