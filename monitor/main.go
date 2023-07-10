package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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

// 格式化前的字段
type Field struct {
	Key    string        // 字段名
	Values []interface{} // key对应的值必须在该列表中，值的元素需要先转化为字符串
}

// 格式化后的字段
type FmtField struct {
	Key    string   // 字段名
	Values []string // 格式化后的字段
}

type URLConfig struct {
	Url       string     // 监控的url
	Method    string     // 方法: get，post等。如果为空，则默认为get
	Any       bool       // 是否只要满足其中一个监控字段即可，默认是所有
	Fields    []Field    // 需要监控的字段
	fmtFields []FmtField // 格式化后的字段
}

// 接口请求响应值的标注化格式
type RespDict map[string]string

// 检测不通过的URL
type ErrUrl struct {
	Url string `json:"url"` //
	Msg string `json:"msg"` // 错误信息
}

var errUrls []ErrUrl

func main() {
	errUrls = []ErrUrl{}
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(fmt.Errorf("配置文件%s读取出错: %s", configFile, err.Error()))
	}
	// println(string(data))
	var urls []URLConfig
	err = json.Unmarshal(data, &urls)
	if err != nil {
		fmt.Println("从配置文件读取的原始数据:", len(data))
		panic(fmt.Errorf("配置文件%s数据转换出错: %s", configFile, err.Error()))
	}
	for idx, url := range urls {
		if len(url.Url) < 5 {
			panic(fmt.Errorf("检测url不应该为空: %d: %v", idx, url))
		}
		// 格式化字段
		for _, field := range url.Fields {
			fmtField := FmtField{
				Key:    field.Key,
				Values: []string{},
			}
			for _, v := range field.Values {
				fmtField.Values = append(fmtField.Values, fmt.Sprint(v))
			}
			url.fmtFields = append(url.fmtFields, fmtField)
		}
		// fmt.Printf("配置数据：%+v\n", url)
		checkURL(&url)
	}
	errBytes, err := json.Marshal(errUrls)
	if err != nil {
		panic(fmt.Errorf("转换成json出错: %s", err.Error()))
	}
	errText := string(errBytes)
	fmt.Println(errText)
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
	if len(url.fmtFields) == 0 {
		// 如果fields为空，则只要状态码为200则算通过
		return
	}

	// 判断字段
	isAll, isAny := true, false
	for _, field := range url.fmtFields {
		ok := checkField(&field, resp)
		if !ok {
			isAll = false
		} else {
			isAny = true
		}
	}
	// print(isAll, isAny, url.Any)
	if url.Any && isAny {
		// 只要满足一个条件
		return
	} else if !url.Any && isAll {
		// 所有条件都满足
		return
	}
	errPrint(url, fmt.Sprintf("响应值(%s)和配置值(%v)不匹配", text, url.Fields))
}

// 请求url并格式化返回值
func request(url *URLConfig) (err error, respDict RespDict, text string) {
	url.Method = strings.ToLower(url.Method)
	if url.Method == "" {
		url.Method = methodGet
	}
	var resp *http.Response
	if url.Method == methodGet {
		resp, err = http.Get(url.Url)
		if err != nil {
			return
		}
		defer resp.Body.Close()
	} else if url.Method == methodPost {
		var body io.Reader
		resp, err = http.Post(url.Url, "", body)
		if err != nil {
			return
		}
		defer resp.Body.Close()
	}
	// 监控http状态码
	if !inList(resp.StatusCode) {
		err = fmt.Errorf("响应状态码%d错误，正确状态码：%v", resp.StatusCode, okStatusList)
		return
	}
	if url.fmtFields != nil {
		err, text, respDict = checkResp(resp)
	}
	return
}

// 检验响应值
func checkResp(resp *http.Response) (err error, text string, respDict RespDict) {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	text = string(body)
	// println("接口响应数据", text)
	respTmp := make(map[string]interface{})
	err = json.Unmarshal(body, &respTmp)
	if err != nil {
		return
	}
	respDict = make(RespDict)
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
func checkField(field *FmtField, resp RespDict) (ok bool) {
	if val, isIn := resp[field.Key]; isIn {
		if len(field.Values) == 0 {
			// 如果values为空，则只需要存在key则满足
			ok = true
			return
		}
		for _, v := range field.Values {
			if v == val {
				ok = true
				return
			}
		}
	}
	return
}

// 触发url检测异常输出
func errPrint(url *URLConfig, msg string) {
	fmt.Printf("异常URL: %s\n====> %s\n", url.Url, msg)
	errUrl := ErrUrl{
		Url: url.Url,
		Msg: msg,
	}
	errUrls = append(errUrls, errUrl)
}
