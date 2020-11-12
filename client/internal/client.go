package internal

import (
	"context"
	"crypto/tls"
	pb "gitlab.com/wjbbig/grpc-stream/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"time"
)

const (
	dialTimeout        = 10 * time.Second
	maxRecvMessageSize = 100 * 1024 * 1024
	maxSendMessageSize = 100 * 1024 * 1024
)

// NewClientConn 创建一个新的感染grpc链接
func NewClientConn(address string, tlsConf *tls.Config,
	kaOpts keepalive.ClientParameters) (*grpc.ClientConn, error) {

	dialOpts := []grpc.DialOption{
		grpc.WithKeepaliveParams(kaOpts),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(maxSendMessageSize),
			grpc.MaxCallRecvMsgSize(maxRecvMessageSize)),
	}

	// 配置TLS
	if tlsConf != nil {
		creds := credentials.NewTLS(tlsConf)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	return grpc.DialContext(ctx, address, dialOpts...)
}

// NewRegisterClient 创建一个新的客户端
func NewRegisterClient(conn *grpc.ClientConn) (pb.ContactSupport_RegisterClient, error) {
	return pb.NewContactSupportClient(conn).Register(context.Background())
}