//go:build linux
// +build linux

package sys

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
)

func HostProc() string {
	return "/proc"
}

func getLinesNum(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	sum := 0
	buf := make([]byte, 8192)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return sum, err
		}
		if n == 0 {
			break
		}

		var buffPosition int
		for {
			i := bytes.IndexByte(buf[buffPosition:n], '\n')
			if i < 0 || n == buffPosition {
				break
			}
			buffPosition += i + 1
			sum++
		}

		if err == io.EOF {
			break
		}
	}
	return sum, nil
}

func GetTCPCount() (int, error) {
	procPath := "/proc"
	tcp4Count := 0
	tcp6Count := 0

	// 获取IPv4 TCP连接数
	tcp4Path := filepath.Join(procPath, "net/tcp")
	count, err := getLinesNum(tcp4Path)
	if err == nil {
		tcp4Count = count - 1 // 减去标题行
	}

	// 获取IPv6 TCP连接数
	tcp6Path := filepath.Join(procPath, "net/tcp6")
	count, err = getLinesNum(tcp6Path)
	if err == nil {
		tcp6Count = count - 1 // 减去标题行
	}

	return tcp4Count + tcp6Count, nil
}

func GetUDPCount() (int, error) {
	procPath := "/proc"
	udp4Count := 0
	udp6Count := 0

	// 获取IPv4 UDP连接数
	udp4Path := filepath.Join(procPath, "net/udp")
	count, err := getLinesNum(udp4Path)
	if err == nil {
		udp4Count = count - 1 // 减去标题行
	}

	// 获取IPv6 UDP连接数
	udp6Path := filepath.Join(procPath, "net/udp6")
	count, err = getLinesNum(udp6Path)
	if err == nil {
		udp6Count = count - 1 // 减去标题行
	}

	return udp4Count + udp6Count, nil
}
