package util

import (
	"flag"
	"github.com/levigross/grequests"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	// 命令行参数
	M3u8UrlFlag              = flag.String("u", "", "m3u8下载地址(http(s)://url/xx/xx/index.m3u8)")
	maxWorkersFlag           = flag.Int("n", 16, "下载线程数(max goroutines num)")
	hostTypeFlag             = flag.String("ht", "apiv1", "设置getHost的方式(apiv1: `http(s):// + url.Host + path.Dir(url.Path)`; apiv2: `http(s)://+ u.Host`")
	DownloadFileName         = flag.String("o", "output", "自定义文件名(默认为output)")
	cookieFlag               = flag.String("c", "", "自定义请求cookie")
	safetyFlag               = flag.Bool("s", false, "是否允许不安全的请求,默认为false")
	SaveDirPath              = flag.String("sp", GetWorkDir(), "文件保存路径,默认为当前路径")
	ffmpegPath               = flag.String("ff", "", "ffmpeg命令路径(默认不使用ffmpeg来进行ts文件合并)")
	highBandWidthFlag        = flag.Bool("hbw", true, "嵌套m3u8下载最高码率的资源,默认true")
	retryTimes               = flag.Int("rt", 5, "单个分片下载最大重试次数,默认5次")
	FFmpegTsFileListFileName = "ffmpeg_ts_file_list.txt"
	MergeFileName            = "merge.mp4"
	logger                   = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	requestOptions           = &grequests.RequestOptions{
		UserAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.88 Safari/537.36",
		RequestTimeout: 20 * time.Second,
		Headers: map[string]string{
			"Connection":      "keep-alive",
			"Accept":          "*/*",
			"Accept-Encoding": "*",
			"Accept-Language": "zh-CN,zh;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5",
		},
	}
)

// InitConfigFromFlag 使用Flag初始化程序相关配置参数
func InitConfigFromFlag() {
	// 解析命令行参数
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	//m3u8播放链接检查
	CheckM3u8Link(M3u8UrlFlag)
	// http 自定义 cookie
	if *cookieFlag != "" {
		requestOptions.Headers["Cookie"] = *cookieFlag
	}
	//ffmpeg检查
	if *ffmpegPath != "" {
		ffmpegCheck(ffmpegPath)
	}
	if !strings.HasPrefix(*M3u8UrlFlag, "http") || !strings.Contains(*M3u8UrlFlag, "m3u8") || *M3u8UrlFlag == "" {
		flag.Usage()
		return
	}
	requestOptions.InsecureSkipVerify = *safetyFlag
}
