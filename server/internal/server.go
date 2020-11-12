package internal

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"net"
	"sync"
)

type GRPCServer struct {
	// 服务端监听地址
	address string
	// 用于处理网络请求
	listener net.Listener
	// grpc服务端
	server *grpc.Server

	lock sync.Mutex
	// TLS配置
	tls *tls.Config
	// 服务端grpc健康检查
	healthServer *health.Server
}

func NewGRPCServer(serverConfig *ServerConfig) (*GRPCServer, error) {
	address := serverConfig.Port
	if address == "" {
		return nil, errors.New("missing address parameter")
	}
	listen, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	return NewGRPCServerFromListener(listen, serverConfig)
}

func NewGRPCServerFromListener(listener net.Listener, serverConfig *ServerConfig) (*GRPCServer, error) {
	grpcServer := &GRPCServer{
		address: listener.Addr().String(),
		listener: listener,
	}

	// 设置server配置
	var serverOpts []grpc.ServerOption

	secOpts := serverConfig.SecOpts
	if secOpts.TLSEnable {
		// 加载tls配置
		cert, err := tls.X509KeyPair(secOpts.ServerCert, secOpts.ServerKey)
		if err != nil {
			return nil, err
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(secOpts.CaCert)

		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    certPool,
		})

		serverOpts = append(serverOpts, grpc.Creds(creds))
	}
	// 设置最大发送和接收消息的大小
	serverOpts = append(serverOpts, grpc.MaxSendMsgSize(MaxSendMsgSize))
	serverOpts = append(serverOpts, grpc.MaxRecvMsgSize(MaxRecvMsgSize))
	// 设置心跳信息
	serverOpts = append(serverOpts, ServerKeepaliveOptions(serverConfig.KaOpts)...)
	serverOpts = append(serverOpts, grpc.ConnectionTimeout(serverConfig.ConnectionTimeout))

	grpcServer.server = grpc.NewServer(serverOpts...)

	// TODO 添加健康检查

	return grpcServer, nil
}

func (server *GRPCServer) Address() string {
	return server.address
}

func (server *GRPCServer) Listener() net.Listener {
	return server.listener
}

func (server *GRPCServer) Server() *grpc.Server {
	return server.server
}

func (server *GRPCServer) TLSEnabled() bool {
	return server.tls != nil
}

// Start 启动服务
func (server *GRPCServer) Start() error {
	return server.server.Serve(server.listener)
}

func (server *GRPCServer) Stop() {
	server.server.Stop()
}

