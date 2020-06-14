package exapi

import (
	"fmt"
	"log"
)

// 日志接口，外部系统可以使用此接口来替换默认日志
type Logger interface {
	Write(string)
}

// 默认日志接口
var defaultLogger Logger

// 日志开关，默认为打开状态
var logOn bool = true

// 设置自定义日志接口，只需要实现接口 Write(string)
func SetLogger(l Logger) {
	defaultLogger = l
}

// 开启日志
func OpenLog() {
	logOn = true
}

// 关闭日志
func CloseLog() {
	logOn = false
}

// 获取日志状态
func IsLoging() bool {
	return logOn
}

// 打印日志
func Log(format string, v ...interface{}) {
	if logOn {
		if defaultLogger != nil {
			defaultLogger.Write(fmt.Sprintf(format+"\n", v...))
		} else {
			log.Printf(format+"\n", v...)
		}
	}
}

// 打印错误日志，不可关闭
func Error(format string, v ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Write(fmt.Sprintf(format+"\n", v...))
	} else {
		log.Printf(format+"\n", v...)
	}
}
