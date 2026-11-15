// Command outfetch 是内部的出方向抓取服务。
//
// 业务只想拿到"这个 URL 能不能访问、有多大、内容摘要是什么"，
// 不落盘、不回传正文，所以对外只暴露 POST /fetch。
package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/outfetch/internal/api"
	"example.com/outfetch/internal/fetcher"
	"example.com/outfetch/internal/policy"
)

func main() {
	addr := flag.String("addr", ":8088", "监听地址")
	allowed := flag.String("allow", "", "允许的主机，逗号分隔；留空表示按默认的私网拒绝策略")
	blocked := flag.String("block", "", "拒绝的主机，逗号分隔")
	ports := flag.String("ports", "80,443", "允许的端口，逗号分隔")
	maxRedirects := flag.Int("max-redirects", 3, "最多跟随几跳重定向")
	maxBytes := flag.Int64("max-bytes", 1<<20, "响应体最多读多少字节")
	flag.Parse()

	p := policy.New(splitCSV(*allowed), splitCSV(*blocked), parseInts(*ports), nil)

	f := fetcher.New(p)
	f.MaxRedirects = *maxRedirects
	f.MaxBytes = *maxBytes

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(p, f).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("outfetch listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parseInts(s string) []int {
	var out []int
	for _, part := range splitCSV(s) {
		if n, err := strconv.Atoi(part); err == nil {
			out = append(out, n)
		}
	}
	return out
}
