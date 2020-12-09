package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	jsoniter "github.com/json-iterator/go"
	"reflect"
	"strings"
	"time"
)

// 统一响应格式
type ResultWithTxId struct {
	Count   int         `json:"count"`  // 数据个数计数
	TxId    string      `json:"txId"`   // 事务ID
	TxTime  int64       `json:"txTime"` // 事务时间戳
	Value   interface{} `json:"value"`  // 响应数据
	Message string      `json:"msg"`    // 消息
}

// 检查key是否为空，如果为空则报告错误。可以指定选择指定一个keyName用于描述key,默认为"key"
func CheckKeyValid(key string, keyName ...string) error {
	name := "key"
	if len(keyName) == 1 {
		name = keyName[0]
	}
	if key == "" {
		return GetErr(fmt.Sprintf("%s 不能为空!", name))
	}
	return nil
}

// 包装错误信息
func GetErr(msg string) error {
	fmt.Println("错误：", msg)
	return errors.New(msg)
}

// 包装数据响应
func GetReturn(stub shim.ChaincodeStubInterface, msg string, value interface{}) (*ResultWithTxId, error) {
	// 获取事务时间戳
	txTime, err := stub.GetTxTimestamp()
	if err != nil {
		return nil, errors.New("时间戳获取错误")
	}
	var count int
	// 判断value类型
	// 如果value为唯一值，count计为1
	// 如果value为复数值，count计为数据个数
	// 如果value不存在，count计为0
	if !reflect.ValueOf(value).IsValid() {
		value = ""
	} else if reflect.TypeOf(value).Kind() != reflect.String && reflect.ValueOf(value).IsNil() {
		value = ""
	} else if reflect.TypeOf(value).Kind() == reflect.Slice || reflect.TypeOf(value).Kind() == reflect.Array {
		count = reflect.ValueOf(value).Len()
	} else {
		count = 1
	}
	res := &ResultWithTxId{
		TxId:    stub.GetTxID(),
		TxTime:  time.Unix(txTime.Seconds, int64(txTime.Nanos)).Unix(),
		Value:   value,
		Message: msg,
		Count:   count,
	}
	fmt.Printf("%#v\n", res)
	str, _ := jsoniter.MarshalToString(res)
	fmt.Printf("%#v\n", str)
	//fmt.Printf("%#v",string(res.Value.([]uint8)))
	return res, nil
}

// 从账本中获取指定数据
func GetData(ctx contractapi.TransactionContextInterface, key string) ([]byte, error) {
	if err := CheckKeyValid(key); err != nil {
		return nil, err
	}

	// 获取key对应的数据
	dataBytes, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, GetErr("Failed query Profile." + err.Error())
	}
	// 如果数据为空，返回空值
	if dataBytes == nil {
		return nil, nil
	}

	return dataBytes, nil
}

// 删除账本中的数据
func DelData(ctx contractapi.TransactionContextInterface, key string) error {
	if err := CheckKeyValid(key); err != nil {
		return err
	}

	// 获取key对应的数据
	err := ctx.GetStub().DelState(key)
	if err != nil {
		return GetErr("Failed to delete Profile." + err.Error())
	}
	return nil
}

// 将JSON数据存储到账本中
func SaveData(ctx contractapi.TransactionContextInterface, key string, data interface{}) error {
	value, err := jsoniter.Marshal(data)
	if err != nil {
		return GetErr("Failed Marshal Profile." + err.Error())
	}
	// 将数据写入账本
	err = ctx.GetStub().PutState(key, value)
	if err != nil {
		return GetErr("Failed to save Profile." + err.Error())
	}
	return err
}

// 将数据转换为二进制数组
func GetBytes(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// 获取数据对应的hash
func Hash(data interface{}) (string, error) {
	dataBytes, err := GetBytes(data)
	if err != nil {
		return "", GetErr("获取二进制数据错误：" + err.Error())
	}
	sum := sha256.Sum256(dataBytes)
	return fmt.Sprintf("%x", sum), nil
}

// 检测Json对象中required字段是否补全
func CheckRequired(data interface{}) error {
	jsonType := reflect.TypeOf(data)
	jsonValue := reflect.ValueOf(data)
	e := jsonType.Elem()
	for i := 0; i < e.NumField(); i++ {
		jsonTag := e.Field(i).Tag.Get("json")
		if strings.Contains(jsonTag, "required") {
			f := jsonValue.Elem().Field(i)
			miss := false
			switch f.Type().String() {
			case "string":
				if f.Len() == 0 {
					miss = true
				}
			case "int":
				if f.IsZero() {
					miss = true
				}
			case "float64":
				if f.IsZero() {
					miss = true
				}
			default:
				if f.IsNil() {
					miss = true
				}
			}
			if miss {
				return GetErr(fmt.Sprintf("字段%s为必须，请检查", e.Field(i).Name))
			}
		}
	}
	return nil
}

// 将字符串转换为二进制数组, 如果dataStr已经是二进制数字，则返回本身
func StringToBytes(dataStr interface{}) ([]byte, error) {
	var newDataBin []byte
	switch dataStr.(type) {
	case string:
		newDataBin = []byte(dataStr.(string))
	case []byte:
		newDataBin = dataStr.([]byte)
	default:
		return nil, GetErr(fmt.Sprintf("dataStr类型错误（%s），必须为string或[]byte", reflect.TypeOf(dataStr).String()))
	}
	if newDataBin == nil || len(newDataBin) == 0 {
		return nil, GetErr("dataStr不能为空")
	}
	return newDataBin, nil
}

func QueryList(ctx contractapi.TransactionContextInterface, query string, callBack func(data []byte, key string) (interface{}, error)) ([]interface{}, error) {
	var list []interface{}
	// 查询warnings
	qres, err := ctx.GetStub().GetQueryResult(query)
	if err != nil {
		return nil, GetErr("查询出错：" + err.Error())
	}
	defer qres.Close()
	for qres.HasNext() {
		response, err := qres.Next()
		if err != nil {
			return nil, GetErr("qres.Next err." + err.Error())
		}
		data, err := callBack(response.Value, response.Key)
		if err != nil {
			return nil, err
		}
		list = append(list, data)
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list, nil
}
