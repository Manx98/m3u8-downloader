package util

import (
	"bytes"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"io/ioutil"
	"os/exec"
	"runtime"
	"strings"
)

// ==============================shell相关代码====================================

// GbkToUtf8 GBK编码转换为UTF8编码
func GbkToUtf8(s []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(s), simplifiedchinese.GBK.NewDecoder())
	d, e := ioutil.ReadAll(reader)
	if e != nil {
		return nil, e
	}
	return d, nil
}

// 异步打印控制台输出
func asyncLog(reader io.ReadCloser) {
	buf := make([]byte, 1024, 1024)
	for {
		num, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				err = nil
			}
			if err != nil {
				panic(fmt.Errorf("asyncLog 异常:%v", err))
			}
		}
		if num > 0 {
			oByte := buf[:num]
			if runtime.GOOS == "windows" {
				o, e := GbkToUtf8(oByte)
				if e != nil {
					fmt.Printf("编码转换失败:%v\n", e)
				} else {
					oByte = o
				}
			}
			fmt.Print(string(oByte))
		}
	}
}

// 执行控制台命令并实时打印
func execute(cmd *exec.Cmd) {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		panic(fmt.Errorf("开始执行命令出现异常:%v", err))
	}

	go asyncLog(stdout)
	go asyncLog(stderr)

	if err := cmd.Wait(); err != nil {
		panic(fmt.Errorf("等待命令执行出现异常:%v", err))
	}
}

// ExecUnixShell 执行Unix shell
func ExecUnixShell(s string) {
	defer func() {
		err := recover()
		if err != nil {
			panic(fmt.Errorf("执行UnixShell命令[%v]时出现异常", s))
		}
	}()
	cmd := exec.Command("/bin/bash", "-c", s)
	execute(cmd)
}

// ExecWinShell  执行Windows shell
func ExecWinShell(s string) {
	defer func() {
		err := recover()
		if err != nil {
			panic(fmt.Errorf("执行WinShell命令[%v]时出现异常", s))
		}
	}()
	cmd := exec.Command("cmd", "/C", s)
	execute(cmd)
}
