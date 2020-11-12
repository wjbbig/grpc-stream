package main

import (
	"encoding/json"
	"gitlab.com/wjbbig/grpc-stream/client"
	pb "gitlab.com/wjbbig/grpc-stream/proto"
	"log"
	"strconv"
)

type Account struct {
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
}

func (a *Account) Init(stub client.StubInterface) pb.Response {
	return client.Success(nil)
}

func (a *Account) Invoke(stub client.StubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	log.Println(function)
	switch function {
	case "addAccount":
		return a.addAccount(stub, args)
	case "getBalance":
		return a.getBalance(stub, args)
	case "transfer":
		return a.transfer(stub, args)
	default:
		return client.Error("Unsupported method")
	}
}

func (a *Account) addAccount(stub client.StubInterface, args []string) pb.Response {
	if len(args) != 2 {
		return client.Error("Invalid number of arguments. Expecting 2")
	}

	name := args[0]
	balance, _ := strconv.ParseFloat(args[1], 10)
	newAccount := &Account{
		Name:    name,
		Balance: balance,
	}
	data, _ := json.Marshal(newAccount)
	stub.PutState(name, data)

	return client.Success(nil)
}

func (a *Account) getBalance(stub client.StubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return client.Error("Invalid number of arguments. Expecting 1")
	}
	accountBytes, _ := stub.GetState(args[0])
	return client.Success(accountBytes)
}

func (a *Account) transfer(stub client.StubInterface, args []string) pb.Response {
	return client.Error("hai mei you shi xian")
}

func main() {
	a := &Account{}
	log.Println(client.Start(a))
}
