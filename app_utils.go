package main

import (
	"log"
	"os/exec"
	stdruntime "runtime"
)

// openDirectory 打开指定目录（跨平台）
func openDirectory(dir string) error {
	log.Printf("openDirectory: 打开目录: %q", dir)
	var cmd *exec.Cmd
	switch stdruntime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default: // linux
		cmd = exec.Command("xdg-open", dir)
	}
	err := cmd.Start()
	if err != nil {
		log.Printf("openDirectory: 打开目录失败: %v", err)
		return err
	}
	// 释放子进程资源，避免泄漏
	go cmd.Wait()
	return nil
}
