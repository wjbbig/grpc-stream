package client

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"gitlab.com/wjbbig/grpc-stream/client/internal"
	pb "gitlab.com/wjbbig/grpc-stream/proto"
	"io"
)

func clientStreamGetter(config internal.Config) (ClientStream, error) {

	conn, err := internal.NewClientConn(config.Address, config.TLS, config.KaOpts)
	if err != nil {
		return nil, err
	}

	return internal.NewRegisterClient(conn)
}

func Start(op Operation) error {
	// 读取配置文件
	config, err := internal.LoadConfig()
	if err != nil {
		return err
	}

	stream, err := clientStreamGetter(config)
	if err != nil {
		return err
	}
	defer stream.CloseSend()
	return chatWithServer(config.ClientName, stream, op)
}

func chatWithServer(clientName string, stream ServerContactStream, op Operation) error {
	// 创建一个handler用于处理消息
	handler := newHandler(stream, op)
	// 向Server端注册自己
	chaincodeId := &pb.ClientName{Name: clientName}
	payload, err := proto.Marshal(chaincodeId)
	if err != nil {
		return errors.WithMessage(err, "failed to marshal chaincodeId during client registration.")
	}

	if err = handler.serialSend(&pb.ContactMessage{Type: pb.ContactMessage_Type_REGISTER, Payload: payload}); err != nil {
		return errors.WithMessage(err, "failed to send client registration")
	}

	// 用于保存gRPC的消息
	type recvMsg struct {
		msg *pb.ContactMessage
		err error
	}
	msgAvail := make(chan *recvMsg, 1)
	errc := make(chan error)
	// 从stream中读取消息的方法
	receiveMessage := func() {
		in, err := stream.Recv()
		msgAvail <- &recvMsg{in, err}
	}

	go receiveMessage()
	for {
		select {
		case rmsg := <-msgAvail:
			switch {
			case rmsg.err == io.EOF:
				return errors.New("received EOF, ending chaincode stream")
			case rmsg.err != nil:
				err := errors.WithMessage(rmsg.err, "receive failed.")
				return err
			case rmsg.msg == nil:
				err := errors.New("received nil message, ending chaincode stream")
				return err
			default:
				err := handler.handleMessage(rmsg.msg, errc)
				if err != nil {
					return errors.WithMessage(err, "error handling message.")
				}

				go receiveMessage()
			}
		case sendErr := <-errc:
			if sendErr != nil {
				err := fmt.Errorf("error sending: %s", sendErr)
				return err
			}
		}
	}
}
