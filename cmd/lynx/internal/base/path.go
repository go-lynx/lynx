package base

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

// lynxHome 获取 Lynx 工具的主目录。
// 若主目录不存在，则创建该目录。
// 返回 Lynx 工具主目录的路径。
func lynxHome() string {
	// 获取当前用户的主目录
	dir, err := os.UserHomeDir()
	if err != nil {
		// 若获取失败，记录错误并终止程序
		log.Fatal(err)
	}
	// 拼接 Lynx 工具主目录的路径
	home := filepath.Join(dir, ".lynx")
	// 检查主目录是否存在
	if _, err := os.Stat(home); os.IsNotExist(err) {
		// 若不存在，则递归创建目录
		if err := os.MkdirAll(home, 0o700); err != nil {
			// 若创建失败，记录错误并终止程序
			log.Fatal(err)
		}
	}
	return home
}

// lynxHomeWithDir 获取 Lynx 工具主目录下指定子目录的路径。
// 若子目录不存在，则创建该目录。
// 参数 dir 是指定的子目录名称。
// 返回 Lynx 工具主目录下指定子目录的路径。
func lynxHomeWithDir(dir string) string {
	// 拼接 Lynx 工具主目录下指定子目录的路径
	home := filepath.Join(lynxHome(), dir)
	// 检查子目录是否存在
	if _, err := os.Stat(home); os.IsNotExist(err) {
		// 若不存在，则递归创建目录
		if err := os.MkdirAll(home, 0o700); err != nil {
			// 若创建失败，记录错误并终止程序
			log.Fatal(err)
		}
	}
	return home
}

// copyFile 将源文件复制到目标文件，并根据替换规则替换文件内容。
// 参数 src 是源文件路径，dst 是目标文件路径，replaces 是替换规则列表，格式为 [old1, new1, old2, new2, ...]。
// 返回复制过程中可能出现的错误。
func copyFile(src, dst string, replaces []string) error {
	// 获取源文件的信息
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	// 读取源文件的内容
	buf, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// 简单启发式：如果包含 NUL，视为二进制，跳过替换
	if bytes.IndexByte(buf, 0) == -1 && len(replaces) > 0 {
		var old string
		// 遍历替换规则列表
		for i, next := range replaces {
			if i%2 == 0 {
				// 偶数索引的元素为旧字符串
				old = next
				continue
			}
			// 奇数索引的元素为新字符串，进行全局替换
			buf = bytes.ReplaceAll(buf, []byte(old), []byte(next))
		}
	}
	// 将替换后的内容写入目标文件，并保持文件权限不变
	return os.WriteFile(dst, buf, srcInfo.Mode())
}

// copyDir 递归复制源目录到目标目录，并根据替换规则替换文件内容，同时忽略指定的文件或目录。
// 参数 src 是源目录路径，dst 是目标目录路径，replaces 是替换规则列表，ignores 是需要忽略的文件或目录列表。
// 返回复制过程中可能出现的错误。
func copyDir(src, dst string, replaces, ignores []string) error {
	// 获取源目录的信息
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	// 递归创建目标目录，并保持目录权限不变
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}
	// 读取源目录下的所有文件和子目录
	fds, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	// 遍历源目录下的所有文件和子目录
	for _, fd := range fds {
		// 检查是否需要忽略当前文件或目录
		if hasSets(fd.Name(), ignores) {
			continue
		}
		// 拼接源文件或子目录的完整路径
		srcFilePath := filepath.Join(src, fd.Name())
		// 拼接目标文件或子目录的完整路径
		dstFilePath := filepath.Join(dst, fd.Name())
		var e error
		if fd.IsDir() {
			// 若为目录，则递归调用 copyDir 函数
			e = copyDir(srcFilePath, dstFilePath, replaces, ignores)
		} else {
			// 若为文件，则调用 copyFile 函数
			e = copyFile(srcFilePath, dstFilePath, replaces)
		}
		if e != nil {
			return e
		}
	}
	return nil
}

// hasSets 检查指定的名称是否在给定的集合中。
// 参数 name 是要检查的名称，sets 是集合列表。
// 返回布尔值，表示名称是否在集合中。
func hasSets(name string, sets []string) bool {
	// 遍历集合列表
	for _, ig := range sets {
		if ig == name {
			return true
		}
	}
	return false
}

// Tree 打印指定目录下所有文件的创建信息，包括文件名和文件大小。
// 参数 path 是要遍历的目录路径，dir 是基础目录，用于格式化输出路径。
func Tree(path string, dir string) {
	// 递归遍历指定目录下的所有文件和子目录
	_ = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		// 若没有错误，且文件信息不为空，且不是目录
		if err == nil && info != nil && !info.IsDir() {
			// 打印文件创建信息，包括文件名和文件大小
			fmt.Printf("%s %s (%v bytes)\n", color.GreenString("CREATED"), strings.Replace(path, dir+"/", "", -1), info.Size())
		}
		return nil
	})
}
