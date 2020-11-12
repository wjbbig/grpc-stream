package client

import pb "gitlab.com/wjbbig/grpc-stream/proto"

type Operation interface {
	Init(StubInterface) pb.Response
	Invoke(StubInterface) pb.Response
}

type StubInterface interface {
	// 获取所有的输入参数
	GetArgs() [][]byte
	// 将所有的输入参数转为string类型
	GetStringArgs() []string
	// 从输入参数中拆除方法名和方法参数
	GetFunctionAndParameters() (string, []string)
	// 暂时没有用
	InvokeOperation(clientName string, args [][]byte) pb.Response
	// 从server端获取数据
	GetState(key string) ([]byte, error)
	// 将数据保存到server端
	PutState(key string, value []byte) error
	// 从server端删除数据
	DelState(key string) error
	// 获取历史数据
	GetHistoryForKey(key string)
}
