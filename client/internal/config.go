package internal

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"google.golang.org/grpc/keepalive"
	"io/ioutil"
	"time"
)

type Config struct {
	ClientName string
	Address    string
	TLS        *tls.Config
	KaOpts     keepalive.ClientParameters
}

func LoadConfig() (Config, error) {
	config, err := readConfig()
	if err != nil {
		return Config{}, err
	}
	conf := Config{
		ClientName: config.GetString("client.name"),
		// 这里为固定值
		KaOpts: keepalive.ClientParameters{
			Time:                1 * time.Minute,
			Timeout:             20 * time.Second,
			PermitWithoutStream: true,
		},
	}

	if conf.Address = config.GetString("client.address"); conf.Address == "" {
		conf.Address = "localhost:4321"
	}

	if !config.GetBool("client.tls.enable") {
		return conf, nil
	}
	// 处理TLS
	tlsConfig, err := loadTLSConfig(config)
	if err != nil {
		return Config{}, err
	}
	conf.TLS = tlsConfig

	return conf, nil
}

func readConfig() (*viper.Viper, error) {
	config := viper.New()
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	config.AddConfigPath("./conf/")
	config.AddConfigPath("../conf/")
	err := config.ReadInConfig()
	if err != nil {
		return nil, errors.WithMessage(err, "can not find the config file")
	}

	return config, nil
}

func loadTLSConfig(config *viper.Viper) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(config.GetString("client.tls.cert"), config.GetString("client.tls.key"))
	if err != nil {
		return nil, errors.WithMessage(err, "load tls key and cert failed")
	}

	certPool := x509.NewCertPool()
	ca, _ := ioutil.ReadFile(config.GetString("client.tls.rootcert"))
	certPool.AppendCertsFromPEM(ca)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   "localhost",
		RootCAs:      certPool,
	}, nil
}
