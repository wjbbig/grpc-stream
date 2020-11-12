package main

import (
	"fmt"
	pb "gitlab.com/wjbbig/grpc-stream/proto"
	"gitlab.com/wjbbig/grpc-stream/server"
	"gitlab.com/wjbbig/grpc-stream/server/internal"
	"log"
	"net/http"
	"strings"
)

func main() {
	config, err := internal.LoadConfig()
	if err != nil {
		log.Println(err)
		return
	}

	serverConfig := internal.NewServerConfig(config)
	grpcServer, err := internal.NewGRPCServer(serverConfig)
	if err != nil {
		log.Println(err)
		return
	}
	clientSupport, err := server.NewClientSupport(config.GetString("server.db"))
	if err != nil {
		log.Println(err)
		return
	}
	pb.RegisterContactSupportServer(grpcServer.Server(), clientSupport)
	defer clientSupport.Close()
	defer grpcServer.Stop()
	log.Printf("start grpc server, port %s\n", grpcServer.Address())
	go grpcServer.Start()

	// 一个用于发起调用client的http服务器
	http.HandleFunc("/init", func(writer http.ResponseWriter, request *http.Request) {
		values := request.URL.Query()
		client := values.Get("client")
		args := values.Get("args")
		fmt.Fprintln(writer, client, args)
	})

	http.HandleFunc("/invoke", func(writer http.ResponseWriter, request *http.Request) {
		values := request.URL.Query()
		client := values.Get("client")
		args := values.Get("args")
		log.Printf("input args: %s", args)
		input := &pb.ContactInput{Args: StringToBytes(args)}
		response, err := clientSupport.Execute(client, input)
		if err != nil {
			fmt.Fprintln(writer, err.Error())
		} else {
			fmt.Fprintln(writer,response)
		}
	})
	httpPort := config.GetString("server.http_port")
	log.Printf("start http server, port %s\n", httpPort)
	http.ListenAndServe(httpPort, nil)
}

func StringToBytes(arg string) [][]byte {
	var args [][]byte
	if len(arg) == 1 {
		return append(args, []byte(arg))
	}

	strArgs := strings.Split(arg, ",")
	for _, strArg := range strArgs {
		args = append(args, []byte(strArg))
	}

	return args
}
