package main

import (
	"fmt"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

const version = "v0.1.0"

type MyChaincode struct {
	contractapi.Contract
}

func (cc *MyChaincode) Init(ctx contractapi.TransactionContextInterface) (string, error) {
	return "初始化链码成功，版本号：" + version, nil
}

func (cc *MyChaincode) GetVersion(ctx contractapi.TransactionContextInterface) (string, error) {
	return version, nil
}


func main() {
	cc, err := contractapi.NewChaincode(new(MyChaincode))
	if err != nil {
		panic(err.Error())
	}
	if err := cc.Start(); err != nil {
		fmt.Printf("Error starting Chain Health chaincode: %s\n", err.Error())
	}
}
