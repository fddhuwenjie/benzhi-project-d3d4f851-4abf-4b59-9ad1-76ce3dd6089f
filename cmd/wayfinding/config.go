package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Address  string
	DataDir  string
	SelfTest bool
}

func addressFrom(portEnv, flagValue string, flagSet bool) (string, error) {
	addr := flagValue
	if !flagSet && portEnv != "" {
		port, err := strconv.Atoi(portEnv)
		if err != nil || port < 1024 || port > 65535 {
			return "", errors.New("PORT 必须是 1024 到 65535 的端口号")
		}
		addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("addr 必须包含明确主机和端口: %w", err)
	}
	if strings.TrimSpace(host) == "" {
		return "", errors.New("addr 禁止省略主机")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", errors.New("addr 必须绑定回环主机")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1024 || n > 65535 {
		return "", errors.New("addr 端口必须在 1024 到 65535 之间")
	}
	return addr, nil
}
func defaultDataDir() string {
	if d := os.Getenv("WAYFINDING_DATA_DIR"); d != "" {
		return d
	}
	return "./data"
}
