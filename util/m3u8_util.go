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
	TsList    []TsInfo
	Key       string
	KeyType   int
	ParentUrl *string
	Host      *string
	Mode      int
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
	return u.Scheme + "://" + u.Host
}

// 获取Host以及父级Url
func getHostAndParentUrl(Url *url.URL) (string, string) {
	h := Url.Scheme + "://" + Url.Host
	parent := path.Dir(Url.Path)
	if parent == "." || parent == "/" || parent == "" {
		return h, h
	} else if strings.HasPrefix(parent, "/") {
		return h, h + parent
	} else {
		return h, h + "/" + parent
	}
}

// 修复存在重定向URL
func fixM3U8UrlRedirect(response *grequests.Response, parentUrl, host *string) {
	rsp := response.RawResponse.Request.Response
	if rsp != nil && rsp.StatusCode == 302 {
		u, e := url.Parse(rsp.Header.Get("Location"))
		CheckErr(e)
		*host, *parentUrl = getHostAndParentUrl(u)
	} else {
		_, *parentUrl = getHostAndParentUrl(response.RawResponse.Request.URL)
	}
}

// 获取m3u8地址的内容体
// 添加支持嵌套文件解析
func getM3u8Body(Url, host *string) string {
	fmt.Println("[信息]:正在下载M3U8文件内容")
	var m3u8Text string
	requestUrl := *Url
	for {
		r, err := grequests.Get(requestUrl, requestOptions)
		CheckErr(err)
		fixM3U8UrlRedirect(r, Url, host)
		m3u8Text = r.String()
		x := findBeastDownloadResolution(m3u8Text, *Url, *host)
		if x == "" {
			return m3u8Text
		} else {
			requestUrl = x
		}
	}
}

//判断link是否为完整http链接，否则使用提供的host与子拼接
//ParentUrl 与 host 均为末尾无"/"
func buildM3u8ResourceUrl(link, parentUrl, host string) string {
	if strings.HasPrefix(link, "http") {
		return link
	} else {
		if strings.HasPrefix(link, "/") {
			return host + link
		} else {
			return parentUrl + "/" + link
		}
	}
}

//检查并判断是否含有多资源
//从m3u8文件中查符合策略的资源链接
func findBeastDownloadResolution(m3u8Text, Url, host string) string {
	var bastLink string
	markResource := false
	bandWidth := -1
	for _, line := range strings.Split(m3u8Text, "\n") {
		if line == "" {
			continue
		}
		if markResource {
			bastLink = buildM3u8ResourceUrl(line, Url, host)
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
func getM3u8Key(host, parentUrl, m3u8Text string) string {
	fmt.Println("[信息]:正在查找M3U8密匙")
	lines := strings.Split(m3u8Text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "#EXT-X-KEY") {
			uriPos := strings.Index(line, "URI")
			quotationMarkPos := strings.LastIndex(line, "\"")
			keyUrl := strings.Split(line[uriPos:quotationMarkPos], "\"")[1]
			if !strings.HasPrefix(keyUrl, "http") {
				if strings.HasPrefix(keyUrl, "/") {
					keyUrl = parentUrl + "/" + keyUrl
				} else {
					keyUrl = fmt.Sprintf("%s/%s", host, keyUrl)
				}
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
func getTsList(host, parentUrl, m3u8Text string) (tsList []TsInfo) {
	index := 0
	fundInf := false
	for _, line := range strings.Split(m3u8Text, "\n") {
		if fundInf == false && strings.HasPrefix(line, "#EXTINF") {
			fundInf = true
		} else if fundInf && line != "" {
			index++
			Url := line
			if !strings.HasPrefix(line, "http") {
				if strings.HasPrefix(line, "/") {
					Url = host + line
				} else {
					Url = parentUrl + "/" + line
				}
			}
			tsList = append(tsList, TsInfo{
				Name: fmt.Sprintf("%05d.ts", index),
				Url:  Url,
			})
			fundInf = false
		}
	}
	return
}

// CheckM3u8Link 检查m3u8链接
func CheckM3u8Link(link *string) {
	if !strings.HasPrefix(*link, "http") {
		flag.Usage()
		panic(fmt.Errorf("无效M3U8链接:%v", *link))
	}
}

// DecodeM3u8FileByUrl 从文件Url解析m3u8文件
func DecodeM3u8FileByUrl(Url *string) M3U8FileInfo {
	m3u8FileInfo := M3U8FileInfo{
		ParentUrl: Url,
		Host:      refererFlag,
	}
	m3u8Text := getM3u8Body(m3u8FileInfo.ParentUrl, m3u8FileInfo.Host)
	m3u8FileInfo.TsList = getTsList(*m3u8FileInfo.Host, *m3u8FileInfo.ParentUrl, m3u8Text)
	m3u8FileInfo.KeyType = 0
	m3u8FileInfo.Key = getM3u8Key(*m3u8FileInfo.Host, *m3u8FileInfo.ParentUrl, m3u8Text)
	return m3u8FileInfo
}
