package internal

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"io/ioutil"
	"log"
	"time"
)

var (
	MaxRecvMsgSize = 100 * 1024 * 1024
	MaxSendMsgSize = 100 * 1024 * 1024

	DefaultKeepaliveOptions = KeepaliveOptions{
		ServerInterval:    time.Duration(2) * time.Hour,
		ServerTimeout:     time.Duration(20) * time.Second,
		ServerMinInterval: time.Duration(1) * time.Minute,
	}

	DefaultConnectionTimeout = 5 * time.Second

	DefaultPort = ":50001"
)

type ServerConfig struct {
	// server监听端口
	Port              string
	// 连接client的最长尝试时间
	ConnectionTimeout time.Duration
	// 安全方面的配置
	SecOpts SecureOptions
	// 心跳消息的配置
	KaOpts KeepaliveOptions
}

// NewServerConfig 实例化ServerConfig
func NewServerConfig(config *viper.Viper) *ServerConfig {
	serverConfig := &ServerConfig{}

	port := config.GetString("server.port")
	if port == "" {
		serverConfig.Port = DefaultPort
	} else {
		serverConfig.Port = port
	}

	timeout := config.GetDuration("server.timeout.connectiontimeout")
	if timeout == 0 {
		serverConfig.ConnectionTimeout = DefaultConnectionTimeout
	} else {
		serverConfig.ConnectionTimeout = timeout
	}

	// 读取keepalive部分的配置，若没有设置，则使用默认值
	serverInterval := config.GetDuration("server.keepalive.interval")
	if serverInterval == 0 {
		serverInterval = DefaultKeepaliveOptions.ServerInterval
	}
	serverTimeout := config.GetDuration("server.keepalive.timeout")
	if serverTimeout == 0 {
		serverTimeout = DefaultKeepaliveOptions.ServerTimeout
	}
	serverMinInterval := config.GetDuration("server.keepalive.mininterval")
	if serverMinInterval == 0 {
		serverMinInterval = DefaultKeepaliveOptions.ServerMinInterval
	}
	serverConfig.KaOpts = KeepaliveOptions{
		ServerInterval:    serverInterval,
		ServerTimeout:     serverTimeout,
		ServerMinInterval: serverMinInterval,
	}

	// 不开启tls，直接返回
	if !config.GetBool("server.tls.enable") {
		return serverConfig
	}
	// 读取tls证书文件
	secOpts, err := readInTLSFiles(config)
	if err != nil {
		log.Panicf("read tls files failed, err: %s", err)
	}
	serverConfig.SecOpts = secOpts

	return serverConfig
}

// SecureOptions 是关于Server TLS方面的配置
type SecureOptions struct {
	TLSEnable  bool
	ServerKey  []byte
	ServerCert []byte
	CaCert     []byte
}

// KeepaliveOptions 心跳消息的配置
type KeepaliveOptions struct {
	// ServerInterval是持续时间，在该持续时间之后，如果服务器未从客户端看到任何活动，
	// 它将对客户端执行ping操作以查看其是否仍在运行
	ServerInterval time.Duration
	// ServerTimeout是在发送ping之后服务器关闭连接之前服务器等待客户端响应的持续时间
	ServerTimeout time.Duration
	// ServerMinInterval是客户端ping之间的最短允许时间。 如果客户端发送ping的频率更高，
	// 则服务器将断开它们的连接
	ServerMinInterval time.Duration
}

// LoadConfig 读取配置文件
func LoadConfig() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("config/")
	v.AddConfigPath("../config/")
	v.AddConfigPath("../../config/")
	err := v.ReadInConfig()
	if err != nil {
		return nil, err
	}

	return v, nil
}

// ServerKeepaliveOptions 将KeepaliveOptions转为grpc.ServerOption
func ServerKeepaliveOptions(ka KeepaliveOptions) []grpc.ServerOption {
	var serverOpts []grpc.ServerOption
	kap := keepalive.ServerParameters{
		Time:    ka.ServerInterval,
		Timeout: ka.ServerTimeout,
	}

	serverOpts = append(serverOpts, grpc.KeepaliveParams(kap))
	kep := keepalive.EnforcementPolicy{
		MinTime:             ka.ServerMinInterval,
		PermitWithoutStream: true,
	}
	serverOpts = append(serverOpts, grpc.KeepaliveEnforcementPolicy(kep))
	return serverOpts
}

// readInTLSFiles 从指定的位置读取证书文件，并返回SecureOptions对象
func readInTLSFiles(config *viper.Viper) (SecureOptions, error) {
	var secOpts SecureOptions
	// 读取key
	keyPath := config.GetString("server.tls.key")
	if keyPath == "" {
		return SecureOptions{}, errors.New("tls key path is empty")
	}
	keyBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return SecureOptions{}, err
	}
	secOpts.TLSEnable = true
	secOpts.ServerKey = keyBytes

	// 读取cert
	certPath := config.GetString("server.tls.cert")
	if certPath == "" {
		return SecureOptions{}, errors.New("tls cert path is empty")
	}
	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return SecureOptions{}, err
	}
	secOpts.ServerCert = certBytes

	// 读取rootcert
	caPath := config.GetString("server.tls.rootcert")
	if caPath == "" {
		return SecureOptions{}, errors.New("tls ca path is empty")
	}
	caBytes, err := ioutil.ReadFile(caPath)
	if err != nil {
		return SecureOptions{}, err
	}
	secOpts.CaCert = caBytes
	return secOpts, nil
}
