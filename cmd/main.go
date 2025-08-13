package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 创建应用程序实例
	app := NewApp()

	// 启动应用程序
	if err := app.Start(); err != nil {
		log.Fatalf("启动应用程序失败: %v", err)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号或应用程序结束
	select {
	case sig := <-sigChan:
		log.Printf("收到退出信号: %v", sig)
	case <-func() <-chan struct{} {
		ch := make(chan struct{})
		go func() {
			app.Wait()
			close(ch)
		}()
		return ch
	}():
		log.Println("应用程序主动结束")
	}

	// 优雅关闭
	if err := app.Stop(); err != nil {
		log.Printf("关闭应用程序失败: %v", err)
		os.Exit(1)
	}
}
