package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	addr := flag.String("addr", defaultAddress, "回环监听地址")
	dataDir := flag.String("data", defaultDataDir(), "本地数据目录")
	selftest := flag.Bool("selftest", false, "运行真实 HTTP 完整流程自检后退出")
	flag.Parse()
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			set = true
		}
	})
	resolved, err := addressFrom(os.Getenv("PORT"), *addr, set)
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置错误:", err)
		os.Exit(2)
	}
	cfg := config{Address: resolved, DataDir: *dataDir, SelfTest: *selftest}
	if cfg.SelfTest {
		err = runSelfTest(cfg)
	} else {
		err = runServer(cfg)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "运行失败:", err)
		os.Exit(1)
	}
}
