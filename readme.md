[toc]

# 运行环境

> ubuntu version : 20.04
> go version : 1.19

# 使用方法

## 文件说明

1. go.mod 模块定义
2. go.sum 依赖文件
3. websocket_client_chat.go  源文件
4. /tmp/chatctrl  模拟通讯行为的控制文件

## 流程说明

1. 运行websocket_client_chat.go文件
2. 执行连接远程websocket服务器
3. 监听控制文件内容，进行录音控制。向/tmp/chatctrl传入1为开始录音，传入2为停止录音。
4. 从开始录音到停止录音后，进行录音封装后websocket通讯。并接收响应的音频流播放

## 运行软件

`go run websocket_client_chat.go`

# 修改日志

## V0.0.0-250806

初始版本：
1.支持websocket通讯功能
2.支持录音功能
3.支持流式播放功能

