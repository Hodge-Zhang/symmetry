package main

import (
	"fmt"
	"github.com/quic-go/quic-go/http3"
	"io"
	"log"
	"net/http"
)

func main() {

	client := &http.Client{
		Transport: &http3.RoundTripper{},
	}

	// 发出 HTTP/3 请求
	resp, err := client.Get("https://chrome.com/")
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Response hgeader:\n%s\n", resp.Header)
	fmt.Printf("Response body:\n%s\n", body)
}
