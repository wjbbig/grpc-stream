package server

//
//const (
//	PORT = ":50001"
//)
//
//type server struct {}
//
//func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
//	log.Println("request: ", in.Name)
//	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
//}
//
//func main() {
//	cert, err := tls.LoadX509KeyPair("server/keys/server.pem", "server/keys/server.key")
//	if err != nil {
//		log.Fatal(err)
//	}
//	certPool := x509.NewCertPool()
//	ca, _ := ioutil.ReadFile("server/keys/ca.pem")
//	certPool.AppendCertsFromPEM(ca)
//
//	creds := credentials.NewTLS(&tls.Config{
//		Certificates: []tls.Certificate{cert},
//		ClientAuth: tls.RequireAndVerifyClientCert,
//		ClientCAs: certPool,
//	})
//
//	lis, err := net.Listen("tcp", PORT)
//
//	if err != nil {
//		log.Fatalf("failed to listen: %v", err)
//	}
//
//	s := grpc.NewServer(grpc.Creds(creds))
//	pb.RegisterGreeterServer(s, &server{})
//	log.Println("rpc服务已经开启")
//	s.Serve(lis)
//}
