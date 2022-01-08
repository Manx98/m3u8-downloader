//@author:llychao<lychao_vip@163.com>
//@contributor: Junyi<me@junyi.pw>
//@date:2020-02-18
//@功能:golang m3u8 video Downloader
package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"flag"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/levigross/grequests"
)

const (
	// HEAD_TIMEOUT 请求头超时时间
	HEAD_TIMEOUT = 10 * time.Second
	// PROGRESS_WIDTH 进度条长度
	PROGRESS_WIDTH = 40
)

var (
	// 命令行参数
	urlFlag            = flag.String("u", "", "m3u8下载地址(http(s)://url/xx/xx/index.m3u8)")
	nFlag              = flag.Int("n", 16, "下载线程数(max goroutines num)")
	htFlag             = flag.String("ht", "apiv1", "设置getHost的方式(apiv1: `http(s):// + url.Host + path.Dir(url.Path)`; apiv2: `http(s)://+ u.Host`")
	oFlag              = flag.String("o", "output", "自定义文件名(默认为output)")
	cFlag              = flag.String("c", "", "自定义请求cookie")
	sFlag              = flag.Int("s", 0, "是否允许不安全的请求(默认为0)")
	spFlag             = flag.String("sp", "", "文件保存路径(默认为当前路径)")
	ffmpegPath         = flag.String("ffmpeg", "", "ffmpeg命令路径(默认不使用ffmpeg来进行ts文件合并)")
	ffmpegMergeCmdArgs = "-f concat -safe 0 -i ffmpeg_ts_file_list.txt -y -c copy merge.mp4"
	enableFFmpeg       = false
	logger             *log.Logger
	ro                 = &grequests.RequestOptions{
		UserAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.88 Safari/537.36",
		RequestTimeout: HEAD_TIMEOUT,
		Headers: map[string]string{
			"Connection":      "keep-alive",
			"Accept":          "*/*",
			"Accept-Encoding": "*",
			"Accept-Language": "zh-CN,zh;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5",
		},
	}
)

// TsInfo 用于保存 ts 文件的下载地址和文件名
type TsInfo struct {
	Name string
	Url  string
}

func init() {
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
}

func main() {
	Run()
}

func Run() {
	msgTpl := "[功能]:多线程下载直播流 m3u8 视屏（ts + 合并）\n[提醒]:如果下载失败，请使用 -ht=apiv2 \n[提醒]:如果下载失败，m3u8 地址可能存在嵌套\n[提醒]:如果进度条中途下载失败，可重复执行"
	fmt.Println(msgTpl)
	runtime.GOMAXPROCS(runtime.NumCPU())
	now := time.Now()

	// 解析命令行参数
	flag.Parse()
	m3u8Url := *urlFlag
	maxGoroutines := *nFlag
	hostType := *htFlag
	downloadFileName := *oFlag
	cookie := *cFlag
	insecure := *sFlag
	savePath := *spFlag
	ro.Headers["Referer"] = getHost(m3u8Url, "apiv2")
	if insecure != 0 {
		ro.InsecureSkipVerify = true
	}
	// http 自定义 cookie
	if cookie != "" {
		ro.Headers["Cookie"] = cookie
	}

	if !strings.HasPrefix(m3u8Url, "http") || !strings.Contains(m3u8Url, "m3u8") || m3u8Url == "" {
		flag.Usage()
		return
	}
	pwd, _ := os.Getwd()
	if savePath != "" {
		pwd = savePath
	}

	// 判断是否填写ffmpeg参数
	if *ffmpegPath != "" {
		FFmpegCheck()
		enableFFmpeg = true
	}
	//pwd = "/Users/chao/Desktop" //自定义地址
	downloadTmpDir := path.Join(pwd, downloadFileName+"_tmp")
	if isExist, _ := PathExists(downloadTmpDir); !isExist {
		err := os.MkdirAll(downloadTmpDir, os.ModePerm)
		if err != nil {
			panic(fmt.Errorf("创建目录[%v]出现异常:%v", downloadTmpDir, err))
		}
	}

	m3u8Host := getHost(m3u8Url, hostType)
	m3u8Body := getM3u8Body(m3u8Url)
	//m3u8Body := getFromFile()

	tsKey := getM3u8Key(m3u8Host, m3u8Body)
	if tsKey != "" {
		fmt.Printf("待解密 ts 文件 key : %s \n", tsKey)
	}

	tsList := getTsList(m3u8Host, m3u8Body)
	if enableFFmpeg {
		writeFFmpegTsFilePathList(downloadTmpDir, tsList)
	}
	fmt.Println("待下载 ts 文件数量:", len(tsList))

	// 下载ts
	downloader(tsList, maxGoroutines, downloadTmpDir, tsKey)
	fmt.Println("[合并]:正在开始合并ts文件合并到临时文件merge.mp4")
	switch runtime.GOOS {
	case "windows":
		winMergeFile(downloadTmpDir)
	default:
		unixMergeFile(downloadTmpDir)
	}
	fmt.Println("[重命名]:将临时文件重命名为正确文件名")
	finalSavePath := path.Join(pwd, downloadFileName+".mp4")
	err := os.Rename(path.Join(downloadTmpDir, "merge.mp4"), finalSavePath)
	if err != nil {
		panic(fmt.Errorf("重命名合并文件时出现异常:%v", err))
	}
	Chdir(pwd)
	fmt.Println("[清理]开始移除临时缓存目录")
	err = os.RemoveAll(downloadTmpDir)
	if err != nil {
		panic(fmt.Errorf("删除临时目录[%v]时出现错误:%v", downloadTmpDir, err))
	}
	fmt.Printf("[Success] 下载保存路径：%s | 共耗时: %6.2fs\n", finalSavePath, time.Now().Sub(now).Seconds())
}

// writeFFmpegTsFilePathList 将ts文件列表写入到指定目录下的ffmpeg_ts_file_list.txt文件中
// 以便ffmpeg根据其信息来合并ts文件
func writeFFmpegTsFilePathList(downloadTmpDir string, tsList []TsInfo) {
	ffmpegTsListFile := path.Join(downloadTmpDir, "ffmpeg_ts_file_list.txt")
	file, e := os.OpenFile(ffmpegTsListFile, os.O_WRONLY|os.O_CREATE, 0777)
	if e != nil {
		panic(fmt.Errorf("创建[%v]出现异常:%v", ffmpegTsListFile, e))
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(fmt.Errorf("关闭[%v]出现异常:%v", ffmpegTsListFile, e))
		}
	}(file)
	writer := bufio.NewWriter(file)
	for _, tsInfo := range tsList {
		_, err := writer.WriteString("file '" + path.Join(downloadTmpDir, tsInfo.Name) + "'\n")
		if err != nil {
			panic(fmt.Errorf("写入ts文件列表到[%v]时出现异常:%v", ffmpegTsListFile, err))
		}
	}
	err := writer.Flush()
	if err != nil {
		panic(fmt.Errorf("写入ts文件列表到[%v]时出现异常:%v", ffmpegTsListFile, err))
	}
}

// 获取m3u8地址的host
func getHost(Url, ht string) (host string) {
	u, err := url.Parse(Url)
	checkErr(err)
	switch ht {
	case "apiv1":
		host = u.Scheme + "://" + u.Host + path.Dir(u.RawPath)
	case "apiv2":
		host = u.Scheme + "://" + u.Host
	}
	return
}

// 获取m3u8地址的内容体
func getM3u8Body(Url string) string {
	r, err := grequests.Get(Url, ro)
	checkErr(err)
	return r.String()
}

// 获取m3u8加密的密钥
func getM3u8Key(host, html string) (key string) {
	lines := strings.Split(html, "\n")
	key = ""
	for _, line := range lines {
		if strings.Contains(line, "#EXT-X-KEY") {
			uriPos := strings.Index(line, "URI")
			quotationMarkPos := strings.LastIndex(line, "\"")
			keyUrl := strings.Split(line[uriPos:quotationMarkPos], "\"")[1]
			if !strings.Contains(line, "http") {
				keyUrl = fmt.Sprintf("%s/%s", host, keyUrl)
			}
			res, err := grequests.Get(keyUrl, ro)
			checkErr(err)
			if res.StatusCode == 200 {
				key = res.String()
			}
		}
	}
	return
}

func getTsList(host, body string) (tsList []TsInfo) {
	lines := strings.Split(body, "\n")
	index := 0
	var ts TsInfo

	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && line != "" {
			//有可能出现的二级嵌套格式的m3u8,请自行转换！
			index++
			if strings.HasPrefix(line, "http") {
				ts = TsInfo{
					Name: fmt.Sprintf("%05d.ts", index),
					Url:  line,
				}
				tsList = append(tsList, ts)
			} else {
				ts = TsInfo{
					Name: fmt.Sprintf("%05d.ts", index),
					Url:  fmt.Sprintf("%s/%s", host, line),
				}
				tsList = append(tsList, ts)
			}
		}
	}
	return
}

// PathExists 判断文件是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getFromFile() string {
	data, _ := ioutil.ReadFile("./ts.txt")
	return string(data)
}

// 下载ts文件
// @modify: 2020-08-13 修复ts格式SyncByte合并不能播放问题
func downloadTsFile(ts TsInfo, downloadDir, key string, retries int) {
	defer func() {
		if r := recover(); r != nil {
			//fmt.Println("网络不稳定，正在进行断点持续下载")
			downloadTsFile(ts, downloadDir, key, retries-1)
		}
	}()

	currPath := fmt.Sprintf("%s/%s", downloadDir, ts.Name)
	if isExist, _ := PathExists(currPath); isExist {
		//logger.Println("[warn] File: " + ts.Name + "already exist")
		return
	}
	res, err := grequests.Get(ts.Url, ro)
	if err != nil || !res.Ok {
		if retries > 0 {
			downloadTsFile(ts, downloadDir, key, retries-1)
			return
		} else {
			//logger.Printf("[warn] File :%s, Retry %d \n", ts.Url, retries-1)
			return
		}
	}

	// 校验长度是否合法
	var origData []byte
	origData = res.Bytes()
	contentLen := 0
	contentLenStr := res.Header.Get("Content-Length")
	if contentLenStr != "" {
		contentLen, _ = strconv.Atoi(contentLenStr)
	}
	if len(origData) == 0 || (contentLen > 0 && len(origData) < contentLen) || res.Error != nil {
		//logger.Println("[warn] File: " + ts.Name + "res origData invalid or err：", res.Error)
		downloadTsFile(ts, downloadDir, key, retries-1)
		return
	}

	// 解密出视频 ts 源文件
	if key != "" {
		//解密 ts 文件，算法：aes 128 cbc pack5
		origData, err = AesDecrypt(origData, []byte(key))
		if err != nil {
			downloadTsFile(ts, downloadDir, key, retries-1)
			return
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
	err = ioutil.WriteFile(currPath, origData, 0666)
	if err != nil {
		panic(fmt.Errorf("向[%v]写入解密后数据时出现异常:%v", currPath, err))
	}
}

// downloader m3u8 下载器
func downloader(tsList []TsInfo, maxGoroutines int, downloadDir string, key string) {
	retry := 5 //单个 ts 下载重试次数
	var wg sync.WaitGroup
	limiter := make(chan struct{}, maxGoroutines) //chan struct 内存占用 0 bool 占用 1
	tsLen := len(tsList)
	downloadCount := 0
	//var writer *bufio.Writer
	for _, ts := range tsList {
		wg.Add(1)
		limiter <- struct{}{}
		go func(ts TsInfo, downloadDir, key string, retryies int) {
			defer func() {
				wg.Done()
				<-limiter
			}()
			downloadTsFile(ts, downloadDir, key, retryies)
			downloadCount++
			DrawProgressBar("Downloading", float32(downloadCount)/float32(tsLen), PROGRESS_WIDTH, ts.Name)
			return
		}(ts, downloadDir, key, retry)
	}
	wg.Wait()
}

// DrawProgressBar 进度条
func DrawProgressBar(prefix string, proportion float32, width int, suffix ...string) {
	pos := int(proportion * float32(width))
	s := fmt.Sprintf("[%s] %s%*s %6.2f%% \t%s",
		prefix, strings.Repeat("■", pos), width-pos, "", proportion*100, strings.Join(suffix, ""))
	fmt.Print("\r" + s)
}

// ============================== shell相关 ==============================

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

// FFmpegCheck 用于检查FFmpeg配置是否正确
func FFmpegCheck() {
	defer func() {
		err := recover()
		if err == nil {
			fmt.Println("[信息]:ffmpeg配置正确")
		} else {
			fmt.Println("[错误]:无法执行通过ffmpeg检查命令,请检查ffmpeg程序路径是否正确")
			panic(fmt.Errorf("执行ffmpeg 检查命令失败"))
		}
	}()
	fmt.Println("[检查]:开始检查ffmpeg配置是否正确")
	if runtime.GOOS == "windows" {
		ExecWinShell(*ffmpegPath + " -version")
	} else {
		ExecUnixShell(*ffmpegPath + " -version")
	}
}

// ExecUnixShell 执行 shell
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

// windows 合并文件
func winMergeFile(path string) {
	Chdir(path)
	if enableFFmpeg {
		ExecWinShell(*ffmpegPath + " " + ffmpegMergeCmdArgs)
	} else {
		ExecWinShell("copy /b *.ts merge.mp4")
	}
}

func Chdir(path string) {
	err := os.Chdir(path)
	if err != nil {
		panic(fmt.Errorf("更改工作目录为指定目录[%v]时出现异常:%v", path, err))
	}
}

func Rename(oldPath, newPath string) {
	err := os.Rename(oldPath, newPath)
	if err != nil {
		panic(fmt.Errorf("重命名文件[%v] -> [%v]时出现异常:%v", oldPath, newPath, err))
	}
}

// unix 合并文件
func unixMergeFile(path string) {
	Chdir(path)
	if enableFFmpeg {
		ExecWinShell(*ffmpegPath + " " + ffmpegMergeCmdArgs)
	} else {
		ExecUnixShell(`cat *.ts >> merge.mp4`)
	}
}

// ============================== 加解密相关 ==============================

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padText...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}

func AesEncrypt(origData, key []byte, ivs ...[]byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	var iv []byte
	if len(ivs) == 0 {
		iv = key
	} else {
		iv = ivs[0]
	}
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv[:blockSize])
	encrypted := make([]byte, len(origData))
	blockMode.CryptBlocks(encrypted, origData)
	return encrypted, nil
}

func AesDecrypt(encrypted, key []byte, ivs ...[]byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	var iv []byte
	if len(ivs) == 0 {
		iv = key
	} else {
		iv = ivs[0]
	}
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(encrypted))
	blockMode.CryptBlocks(origData, encrypted)
	origData = PKCS7UnPadding(origData)
	return origData, nil
}

func checkErr(e error) {
	if e != nil {
		logger.Panic(e)
	}
}
