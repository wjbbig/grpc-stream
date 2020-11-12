/*
grpc-stream server端执行流程

server包中的文件结构：
	active_message.go 用于记录还没执行完的消息，防止同一个消息被多次执行，也可以防止出现重复的MsgID。
	client_support.go 实现了grpc stream中的接口，用于接收client的连接，并为其分配一个handler用于交互。
	handler.go 用于与指定的client进行交互，包括收发消息，处理数据库数据等。
	handler_registry.go 用于记录client与handler的关系。对于每一个client，server都会为其分配一个独立的handler。
	responsenotifier.go 用于追踪执行中的消息返回，将client的返回消息根据MsgId发放到指定的管道中，可以防止消息接收的混乱。
	internal/config.go 用于读取配置文件和server证书。
	internal/server.go 一个通用的grpc_server的封装对象。

server运行流程：
	1. client通过grpc与server建立连接。
	2. client_support调用 HandleClientSteam 为client分配一个handler，
       并调用handler.ProcessStream使用for循环接收client消息。
	3. 建立连接后，client发送REGISTER消息给server，server返回REGISTERED消息给client，
       并再发送READY消息给client，通知其已经准备好。此时，双边都已经进入READY状态。
	4. server调用client_support.Execute来向client发送操作请求。
*/
package server
