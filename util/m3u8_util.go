package util

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/levigross/grequests"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

// TsInfo 用于保存 ts 文件的下载地址和文件名
type TsInfo struct {
	Name string
	Url  string
}

// M3U8FileInfo 用于存储解码后的m3u8文件信息
type M3U8FileInfo struct {
	TsList  []TsInfo
	Key     string
	KeyType int
}

// writeFFmpegTsFilePathList 将ts文件列表写入到指定目录下的ffmpeg_ts_file_list.txt文件中
// 以便ffmpeg根据其信息来合并ts文件
func writeFFmpegTsFilePathList(downloadTmpDir string, tsList []TsInfo) {
	ffmpegTsListFile := path.Join(downloadTmpDir, FFmpegTsFileListFileName)
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
func getHost(Url string) (host string) {
	u, err := url.Parse(Url)
	CheckErr(err)
	switch *hostTypeFlag {
	case "apiv1":
		parent := path.Dir(u.RawPath)
		if !strings.HasPrefix(parent, "/") {
			parent = ""
		}
		host = u.Scheme + "://" + u.Host + parent
	case "apiv2":
		host = u.Scheme + "://" + u.Host
	}
	return
}

// 获取m3u8地址的内容体
// 添加支持嵌套文件解析
func getM3u8Body(Url, host string) string {
	fmt.Println("[信息]:正在下载M3U8文件内容")
	host = strings.TrimSuffix(host, "/")
	var m3u8Text string
	for {
		r, err := grequests.Get(Url, requestOptions)
		CheckErr(err)
		m3u8Text = r.String()
		x := findBeastDownloadResolution(m3u8Text, host)
		if x == "" {
			return m3u8Text
		} else {
			Url = x
		}
	}
}

//判断link是否为完整http链接，否则使用提供的host与子拼接
func buildM3u8ResourceUrl(link, host string) string {
	if strings.HasPrefix(link, "http") {
		return link
	} else {
		if strings.HasPrefix(link, "/") {
			return host + link
		} else {
			return host + "/" + link
		}
	}
}

//检查并判断是否含有多资源
//从m3u8文件中查符合策略的资源链接
func findBeastDownloadResolution(m3u8Text, host string) string {
	var bastLink string
	markResource := false
	bandWidth := -1
	for _, line := range strings.Split(m3u8Text, "\n") {
		if line == "" {
			continue
		}
		if markResource {
			bastLink = buildM3u8ResourceUrl(line, host)
			markResource = false
		} else {
			x, f := getBandWidth(line)
			if f {
				replaceOld := true
				if bastLink != "" {
					if *highBandWidthFlag {
						replaceOld = x > bandWidth
					} else {
						replaceOld = x < bandWidth
					}
				}
				if replaceOld {
					bandWidth = x
					markResource = true
				}
			}
		}
	}
	return bastLink
}

//获取资源码率
//返回[资源码率(默认0), 是否找到资源码率]
func getBandWidth(line string) (int, bool) {
	if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
		line = strings.TrimPrefix(line, "#EXT-X-STREAM-INF:")
		for _, param := range strings.Split(line, ",") {
			split := strings.Split(param, "=")
			if len(split) == 2 && split[0] == "BANDWIDTH" {
				x, err := strconv.Atoi(split[1])
				CheckErr(err)
				return x, true
			}
		}
	}
	return 0, false
}

//获取m3u8加密的密钥
func getM3u8Key(host, m3u8Text string) string {
	fmt.Println("[信息]:正在查找M3U8密匙")
	lines := strings.Split(m3u8Text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "#EXT-X-KEY") {
			uriPos := strings.Index(line, "URI")
			quotationMarkPos := strings.LastIndex(line, "\"")
			keyUrl := strings.Split(line[uriPos:quotationMarkPos], "\"")[1]
			if !strings.Contains(line, "http") {
				keyUrl = fmt.Sprintf("%s/%s", host, keyUrl)
			}
			fmt.Println("[信息]:正在下载解密 ts 文件 Key")
			res, err := grequests.Get(keyUrl, requestOptions)
			CheckErr(err)
			if res.StatusCode == 200 {
				key := res.String()
				fmt.Printf("[信息]:解密 ts 文件 Key : %s \n", key)
				return key
			} else {
				panic(fmt.Errorf("无法下载密匙,错误状态码:%v", res.StatusCode))
			}
		}
	}
	fmt.Println("[信息]:没有发现解密 ts 文件 Key")
	return ""
}

//获取Ts文件列表信息
func getTsList(host, m3u8Text string) (tsList []TsInfo) {
	lines := strings.Split(m3u8Text, "\n")
	index := 0
	var ts TsInfo
	fundInf := false
	for _, line := range lines {
		if fundInf == false && strings.HasPrefix(line, "#EXTINF") {
			fundInf = true
		} else if fundInf && line != "" {
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
			fundInf = false
		}
	}
	return
}

// CheckM3u8Link 检查m3u8链接
func CheckM3u8Link(link *string) {
	if !strings.HasPrefix(*link, "http") || !strings.Contains(*link, "m3u8") || *M3u8UrlFlag == "" {
		flag.Usage()
		panic(fmt.Errorf("无效M3U8链接:%v", *link))
	}
}

// DecodeM3u8FileByContent 从字符串解析m3u8文件
func DecodeM3u8FileByContent(host, m3u8Text string) M3U8FileInfo {
	m3u8FileInfo := M3U8FileInfo{
		Key:     getM3u8Key(host, m3u8Text),
		TsList:  getTsList(host, m3u8Text),
		KeyType: 0,
	}
	m3u8FileInfo.KeyType = 0
	return m3u8FileInfo
}

// DecodeM3u8FileByUrl 从文件Url解析m3u8文件
func DecodeM3u8FileByUrl(url string) M3U8FileInfo {
	host := getHost(url)
	return DecodeM3u8FileByContent(host, getM3u8Body(url, host))
}
