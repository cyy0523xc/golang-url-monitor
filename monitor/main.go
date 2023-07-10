package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// 配置文件路径
const configFile = "./config.json"

// 允许通过的http状态码
var okStatusList = []int{http.StatusOK}

// http方法
const (
	methodGet  = "get"
	methodPost = "post"
)

type Field struct {
	key    string   // 字段名
	values []string // key对应的值必须在该列表中，值的元素需要先转化为字符串
}

type URLConfig struct {
	url    string  // 监控的url
	method string  // 方法: get，post等。如果为空，则默认为get
	any    bool    // 是否只要满足其中一个监控字段即可，默认是所有
	fields []Field // 需要监控的字段
}

// 返回值
type Resp map[string]interface{}

// 转换后的响应值
type RespDict map[string]string

func main() {
	println("开始检测url，配置文件: ", configFile)
	f, err := os.OpenFile(configFile, os.O_RDONLY, 0)
	if err != nil {
		panic(fmt.Errorf("打开配置文件%s出错: %s", configFile, err.Error()))
	}
	defer f.Close()
	var data []byte
	_, err = f.Read(data)
	if err != nil {
		panic(fmt.Errorf("配置文件%s读取出错: %s", configFile, err.Error()))
	}
	var urls []URLConfig
	err = json.Unmarshal(data, &urls)
	if err != nil {
		panic(fmt.Errorf("打开配置文件%s出错: %s", configFile, err.Error()))
	}
	for _, url := range urls {
		checkURL(&url)
	}
}

// 如果fields为空，则只要状态码为200则算通过
// 如果fields不为空，才需要校验key和value
// 如果value为空，则只需要存在key则满足
func checkURL(url *URLConfig) {
	// 请求url
	err, resp, text := request(url)
	if err != nil {
		errPrint(url, err.Error())
		return
	}
	if len(url.fields) == 0 {
		// 如果fields为空，则只要状态码为200则算通过
		return
	}

	// 判断字段
	isAll, isAny := true, false
	for _, field := range url.fields {
		ok := checkField(&field, resp)
		if !ok {
			isAll = false
		} else {
			isAny = true
		}
	}
	if url.any && isAny {
		// 只要有一个命中，则命中
		errPrint(url, text)
	} else if !url.any && isAll {
		// 只要有一个不命中，则命中
		errPrint(url, text)
	}
	return
}

// 请求url并格式化返回值
func request(url *URLConfig) (err error, respDict RespDict, text string) {
	url.method = strings.ToLower(url.method)
	if url.method == "" {
		url.method = methodGet
	}
	var resp *http.Response
	if url.method == methodGet {
		resp, err = http.Get(url.url)
		if err != nil {
			return
		}
	} else if url.method == methodPost {
		var body io.Reader
		resp, err = http.Post(url.url, "", body)
		if err != nil {
			return
		}
	}
	err, text, respDict = fmtResp(resp)
	return
}

// 获取响应值
func fmtResp(resp *http.Response) (err error, text string, respDict RespDict) {
	if !inList(resp.StatusCode) {
		err = fmt.Errorf("响应状态码错误：%x", okStatusList)
		return
	}
	var body []byte
	_, err = resp.Body.Read(body)
	if err != nil {
		return
	}
	text = string(body)
	var respTmp Resp
	err = json.Unmarshal(body, &respTmp)
	if err != nil {
		return
	}
	for k, v := range respTmp {
		respDict[k] = fmt.Sprint(v)
	}
	return
}

// 状态是否在允许列表里
func inList(status int) bool {
	for _, s := range okStatusList {
		if s == status {
			return true
		}
	}
	return false
}

// 判断某个字段是否满足条件
func checkField(field *Field, resp RespDict) (ok bool) {
	if val, isIn := resp[field.key]; isIn {
		if len(field.values) == 0 {
			// 如果values为空，则只需要存在key则满足
			ok = true
			return
		}
		for _, v := range field.values {
			if v == val {
				ok = true
				return
			}
		}
	}
	return
}

// 触发url检测异常输出
func errPrint(url *URLConfig, respText string) {
	println("异常URL：", url.url, "\n ===> ", respText)
}
