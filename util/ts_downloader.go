package util

import (
	"fmt"
	"github.com/levigross/grequests"
	"io/ioutil"
	"path"
	"strconv"
	"sync"
	"time"
)

var ProgressWidth = 40

type DownloadResult struct {
	length int
	err    error
}

func fileDownloadHandler(Url, savePath string, key []byte) (int, error) {
	//在没有校验的情况下此操作无法保证资源完整性
	contentLen := 0
	//todo 未校验文件完整性
	if isExist, _, _ := PathExists(savePath); isExist {
		return contentLen, nil
	}
	res, err := grequests.Get(Url, requestOptions)
	if err != nil {
		return contentLen, err
	}
	if !res.Ok {
		return contentLen, fmt.Errorf("请求失败: %v", Url)
	}
	// 校验长度是否合法
	origData := res.Bytes()
	contentLen, err = strconv.Atoi(res.Header.Get("Content-Length"))
	if err != nil {
		return contentLen, err
	}
	if len(origData) == 0 || (contentLen > 0 && len(origData) < contentLen) || res.Error != nil {
		return contentLen, fmt.Errorf("Content-Length:%v, 但是接收到了:%v", contentLen, len(origData))
	}

	// 解密出视频 ts 源文件
	if key != nil {
		//解密 ts 文件，算法：aes 128 cbc pack5
		//todo 支持多种加密协议
		origData, err = AesDecrypt(origData, []byte(key))
		if err != nil {
			return contentLen, err
		}
	}
	// https://en.wikipedia.org/wiki/MPEG_transport_stream
	// Some TS files do not start with SyncByte 0x47, they can not be played after merging,
	// Need to remove the bytes before the SyncByte 0x47(71).
	syncByte := uint8(71) //0x47
	bLen := len(origData)
	for j := 0; j < bLen; j++ {
		if origData[j] == syncByte {
			origData = origData[j:]
			break
		}
	}
	err = ioutil.WriteFile(savePath, origData, 0666)
	if err != nil {
		panic(fmt.Errorf("向[%v]写入解密后数据时出现异常:%v", savePath, err))
	}
	return contentLen, err
}

// 下载ts文件
// @modify: 2020-08-13 修复ts格式SyncByte合并不能播放问题
func downloadTsFile(ts TsInfo, downloadDir string, key []byte, retries int) int {
	currPath := path.Join(downloadDir, ts.Name)
	for {
		completedLength, err := fileDownloadHandler(ts.Url, currPath, key)
		if err != nil {
			if retries == 0 {
				panic(fmt.Errorf("超过重试最大次数:%v", err))
			} else {
				fmt.Printf("分片[%v]下载出现异常,即将重试:%v\n", ts.Name, err)
			}
			retries -= 1
		} else {
			return completedLength
		}
	}
}

var DataSizeUnit = []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}

func formatDataSize(dataLength float64) string {
	i := 0
	for ; dataLength > 1024 && i < len(DataSizeUnit)-1; i++ {
		dataLength /= 1024
	}
	return fmt.Sprintf("%.2f%v", dataLength, DataSizeUnit[i])
}

// DownloadTsFiles m3u8 下载器
func DownloadTsFiles(m3u8Info M3U8FileInfo, downloadDir string) {
	var wg sync.WaitGroup
	limiter := make(chan struct{}, *maxWorkersFlag) //chan struct 内存占用 0 bool 占用 1
	tsLen := len(m3u8Info.TsList)
	downloadCount := 0
	downloadDataCount := 0
	//var writer *bufio.Writer
	var key []byte
	if m3u8Info.Key != "" {
		key = []byte(m3u8Info.Key)
	}
	now := time.Now()
	for _, ts := range m3u8Info.TsList {
		wg.Add(1)
		limiter <- struct{}{}
		go func(ts TsInfo, downloadDir string, key []byte, retries int) {
			defer func() {
				<-limiter
				wg.Done()
			}()
			downloadDataCount += downloadTsFile(ts, downloadDir, key, retries)
			downloadCount++
			DrawProgressBar("下载", float32(downloadCount)/float32(tsLen), ProgressWidth, ts.Name, "已下载:"+formatDataSize(float64(downloadDataCount)), formatDataSize(float64(downloadDataCount)/time.Now().Sub(now).Seconds())+"/s")
		}(ts, downloadDir, key, *retryTimes)
	}
	wg.Wait()
}
