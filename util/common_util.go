package util

import (
	"fmt"
	"os"
)

// ============== 通用工具 ==================

// Chdir 改变工作目录
func Chdir(path string) {
	err := os.Chdir(path)
	if err != nil {
		panic(fmt.Errorf("更改工作目录为指定目录[%v]时出现异常:%v", path, err))
	}
}

// Rename 重命名文件
func Rename(oldPath, newPath string) {
	err := os.Rename(oldPath, newPath)
	if err != nil {
		panic(fmt.Errorf("重命名文件[%v] -> [%v]时出现异常:%v", oldPath, newPath, err))
	}
}

// CheckErr 异常检查
func CheckErr(e error) {
	if e != nil {
		logger.Panic(e)
	}
}

// GetWorkDir 获取当前工作目录
func GetWorkDir() string {
	pwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("无法获取当前工作目录:%v", err))
	}
	return pwd
}

// PathExists 判断文件是否存在
// 是否存在, 文件大小, 错误
func PathExists(path string) (bool, int64, error) {
	fileInfo, err := os.Stat(path)
	if err == nil {
		return true, fileInfo.Size(), nil
	}
	if os.IsNotExist(err) {
		return false, 0, nil
	}
	return false, 0, err
}
