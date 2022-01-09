//@author:llychao<lychao_vip@163.com>
//@contributor: Junyi<me@junyi.pw>
//@date:2020-02-18
//@功能:golang m3u8 video Downloader
package main

import (
	"fmt"
	"m3u8-downloader/util"
	"os"
	"path"
	"time"
)

func main() {
	Run()
}

func Run() {
	msgTpl := "[功能]:多线程下载直播流 m3u8 视屏（ts + 合并）\n[提醒]:如果下载失败，请使用 -ht=apiv2 \n[提醒]:如果下载失败，m3u8 地址可能存在嵌套\n[提醒]:如果进度条中途下载失败，可重复执行"
	fmt.Println(msgTpl)
	now := time.Now()
	pwd := util.GetWorkDir()
	util.InitConfigFromFlag()
	//pwd = "/Users/chao/Desktop" //自定义地址
	downloadTmpDir := path.Join(*util.SaveDirPath, *util.DownloadFileName+"_tmp")
	isExist, _, err := util.PathExists(downloadTmpDir)
	util.CheckErr(err)
	if !isExist {
		err = os.MkdirAll(downloadTmpDir, os.ModePerm)
		if err != nil {
			panic(fmt.Errorf("创建目录[%v]出现异常:%v", downloadTmpDir, err))
		}
	}
	m3u8Info := util.DecodeM3u8FileByUrl(*util.M3u8UrlFlag)
	if len(m3u8Info.TsList) > 0 {
		fmt.Println("[信息]:待下载 ts 文件数量:", len(m3u8Info.TsList))
		// 下载ts
		util.DownloadTsFiles(m3u8Info, downloadTmpDir)
		fmt.Println("\n[信息]:正在合并ts文件合并到临时文件merge.mp4")
		tempFile := util.AutoMergeTsFile(downloadTmpDir, m3u8Info)
		fmt.Println("[信息]:将临时文件重命名为正确文件名")
		finalSavePath := path.Join(*util.SaveDirPath, *util.DownloadFileName+".mp4")
		util.Rename(tempFile, finalSavePath)
		util.Chdir(pwd)
		fmt.Println("[信息]:开始移除临时缓存目录")
		err = os.RemoveAll(downloadTmpDir)
		if err != nil {
			panic(fmt.Errorf("删除临时目录[%v]时出现错误:%v", downloadTmpDir, err))
		}
		fmt.Printf("[结束]:下载保存路径：%s | 共耗时: %6.2fs\n", finalSavePath, time.Now().Sub(now).Seconds())
	} else {
		fmt.Println("没有发现任何可供下载的分片信息,请检查链接是否正确!")
	}
}
