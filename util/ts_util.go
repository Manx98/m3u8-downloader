package util

import (
	"fmt"
	"path"
	"runtime"
)

// 存放Ts相关操作代码
var ffmpegMergeCmdArgs = "%s -f concat -safe 0 -i %s -y -c copy %s"

// windows 合并文件
// 当ffmpegPath为空时只使用二进制合并
// 只有使用ffmpeg合并文件时才需要tsFileListInfoFilePath
func winMergeFile(workDir string, ffmpegPath, tsFileListInfoFilePath *string) string {
	Chdir(workDir)
	join := path.Join(workDir, MergeFileName)
	if ffmpegPath != nil {
		ExecWinShell(fmt.Sprintf(ffmpegMergeCmdArgs, *ffmpegPath, *tsFileListInfoFilePath, join))
	} else {
		ExecWinShell(fmt.Sprintf("copy /b *.ts \"%s\" /Y", join))
	}
	return join
}

// unix 合并文件
// 当ffmpegPath为空时只使用二进制合并
// 只有使用ffmpeg合并文件时才需要tsFileListInfoFilePath
func unixMergeFile(workDir string, ffmpegPath, tsFileListInfoFilePath *string) string {
	Chdir(workDir)
	join := path.Join(workDir, MergeFileName)
	if ffmpegPath != nil {
		ExecUnixShell(fmt.Sprintf(ffmpegMergeCmdArgs, *ffmpegPath, *tsFileListInfoFilePath, join))
	} else {
		ExecUnixShell(fmt.Sprintf("cat *.ts >> \"%s\"", join))
	}
	return join
}

// 跨平台合并
func mergeHandler(workDir string, ffmpegPath, tsFileListInfoFilePath *string) string {
	if runtime.GOOS == "windows" {
		return winMergeFile(workDir, ffmpegPath, tsFileListInfoFilePath)
	} else {
		return unixMergeFile(workDir, ffmpegPath, tsFileListInfoFilePath)
	}
}

// MergeTsFile 使用二进制方式合并
// 合并完成的文件生成在 tsFileDir 目录下merge.mp4
func MergeTsFile(tsFileDir string) string {
	return mergeHandler(tsFileDir, nil, nil)
}

// MergeTsFileWithFFmpeg 使用ffmpeg合并
// 合并完成的文件生成在 workDir 目录下merge.mp4
func MergeTsFileWithFFmpeg(workDir, ffmpegPath, tsFileListInfoFilePath string) string {
	return mergeHandler(workDir, &ffmpegPath, &tsFileListInfoFilePath)
}

// ffmpegCheck 用于检查FFmpeg配置是否正确
// 检查通过返回输入值,反之触发panic
func ffmpegCheck(ffmpegPath *string) {
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

// AutoMergeTsFile 自动判断使用何种方式合并Ts文件
// 合并成功后返回合并到的文件路径
func AutoMergeTsFile(downloadTempDir string, info M3U8FileInfo) string {
	if *ffmpegPath != "" {
		writeFFmpegTsFilePathList(downloadTempDir, info.TsList)
		return MergeTsFileWithFFmpeg(downloadTempDir, *ffmpegPath, FFmpegTsFileListFileName)
	} else {
		return MergeTsFile(downloadTempDir)
	}
}
