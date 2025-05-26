//go:build windows
// +build windows

package sys

import (
	"os/exec"
	"strings"
)

func GetTCPCount() (int, error) {
	cmd := exec.Command("netstat", "-n")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "TCP") {
			count++
		}
	}
	return count, nil
}

func GetUDPCount() (int, error) {
	cmd := exec.Command("netstat", "-n")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "UDP") {
			count++
		}
	}
	return count, nil
}
