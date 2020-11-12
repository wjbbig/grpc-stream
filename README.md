# grpc-stream

一个用于学习GRPC stream模式的例子。代码来自于Hyperledger fabric2.2中peer与chaincode交互部分，并进行了大量的简化，去除了与grpc无关的代码。

| 依赖   | 版本         |
| ------ | ------------ |
| golang | 1.14或者更高 |

## 使用方法

在`server/impl`中有一个`server`的实例，在`client/impl`中有两个`client`的实例，编译后启动它们即可。`client`的证书在`client/certs`下，所有client都可以共用这些证书，也可以单独生成。