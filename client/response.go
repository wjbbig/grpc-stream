package client

import pb "gitlab.com/wjbbig/grpc-stream/proto"

const (
	OK    = 200
	ERROR = 500
)


// Success ...
func Success(payload []byte) pb.Response {
	return pb.Response{
		Status:  OK,
		Payload: payload,
	}
}

// Error ...
func Error(msg string) pb.Response {
	return pb.Response{
		Status:  ERROR,
		Message: msg,
	}
}


