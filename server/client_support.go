package server

import (
	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	pb "gitlab.com/wjbbig/grpc-stream/proto"
	"log"
	"time"
)

type ClientSupport struct {
	ExecuteTimeout  time.Duration
	Keepalive       time.Duration
	DB              *bolt.DB
	HandlerRegistry *HandlerRegistry
}

func NewClientSupport(dbPath string) (*ClientSupport, error) {

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "open db failed")
	}
	return &ClientSupport{
		ExecuteTimeout:  time.Duration(30) * time.Second,
		Keepalive:       0,
		DB:              db,
		HandlerRegistry: newHandlerRegistry(),
	}, nil
}

func (cs *ClientSupport) HandleClientSteam(stream ClientStream) error {
	handler := &Handler{
		KeepAlive:        cs.Keepalive,
		Registry:         cs.HandlerRegistry,
		stateStore:       cs.DB,
		state:            created,
		activeMsgs:       NewActiveMessages(),
		responseNotifier: NewResponseNotifier(),
	}

	return handler.ProcessStream(stream)
}

// Register 实现grpc接口
func (cs *ClientSupport) Register(stream pb.ContactSupport_RegisterServer) error {
	log.Println("recv connection")
	return cs.HandleClientSteam(stream)
}

func (cs *ClientSupport) Handler(clientName string) (*Handler, error) {
	if h := cs.HandlerRegistry.Handler(clientName); h != nil {
		return h, nil
	}
	return nil, errors.New("client was not been registered")
}

func (cs *ClientSupport) Execute(clientName string, input *pb.ContactInput) (*pb.Response, error) {
	handler, err := cs.Handler(clientName)
	if err != nil {
		return nil, err
	}
	// 生成随机消息ID
	msgId := RandomMsgId()
	// 调用handler执行操作
	resp, err := cs.execute(pb.ContactMessage_Type_INVOKE, msgId, input, handler)
	// 处理结果
	return processExecuteResult(msgId, resp, err)
}

func (cs *ClientSupport) execute(msgType pb.ContactMessage_Type, msgId string, input *pb.ContactInput, h *Handler) (*pb.ContactMessage, error) {
	payload, err := proto.Marshal(input)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create client message")
	}

	msg := &pb.ContactMessage{
		Type:    msgType,
		Payload: payload,
		MsgId:   msgId,
	}

	resp, err := h.Execute(msg, cs.ExecuteTimeout)
	if err != nil {
		return nil, errors.WithMessage(err, "error sending")
	}

	return resp, nil
}

// processExecuteResult 处理执行操作返回的结果
func processExecuteResult(msgId string, resp *pb.ContactMessage, err error) (*pb.Response, error) {
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute message %s", msgId)
	}
	if resp == nil {
		return nil, errors.Errorf("nil response from message %s", msgId)
	}

	switch resp.Type {
	case pb.ContactMessage_Type_ERROR:
		return nil, errors.Errorf("execute returned with failure: %s", resp.Payload)
	case pb.ContactMessage_Type_COMPLETED:
		res := &pb.Response{}
		err := proto.Unmarshal(resp.Payload, res)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal response for message %s", msgId)
		}
		return res, nil
	default:
		return nil, errors.Errorf("unexpected response type %d for message %s", resp.Type, msgId)
	}
}

func RandomMsgId() string {
	return uuid.New().String()
}

func (cs *ClientSupport) Close() {
	cs.DB.Close()
}
