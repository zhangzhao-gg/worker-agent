/**
 * [INPUT]: 依赖 internal/server, internal/city, internal/llm
 * [OUTPUT]: 对外提供 worker-agent 可执行入口
 * [POS]: 进程启动点，初始化依赖，启动 HTTP 服务 + 自动恢复
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"worker-agent/internal/city"
	"worker-agent/internal/llm"
	"worker-agent/internal/server"
)

func main() {
	port := flag.Int("port", 8080, "HTTP 服务端口")
	dataDir := flag.String("data", "./data", "数据目录（存放工人 DB 文件）")
	cityURL := flag.String("city", "", "城市 API 地址（空则启用 mock 模式）")
	llmURL := flag.String("llm-url", "https://api.minimax.chat", "LLM API 地址")
	llmKey := flag.String("llm-key", "", "LLM API Key（默认读 LLM_API_KEY 环境变量）")
	llmModel := flag.String("llm-model", "MiniMax-Text-01", "LLM 模型名称")
	flag.Parse()

	// ── LLM 客户端 ──
	apiKey := *llmKey
	if apiKey == "" {
		apiKey = os.Getenv("LLM_API_KEY")
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "错误: 需要 LLM API Key（-llm-key 或 LLM_API_KEY 环境变量）")
		os.Exit(1)
	}
	llmClient := llm.NewMiniMax(*llmURL, apiKey, *llmModel)

	// ── 城市 API ──
	var cityAPI *city.CityAPI
	if *cityURL == "" {
		log.Println("[main] 城市 API 未配置，启用 mock 模式")
		cityAPI = city.NewMock()
	} else {
		cityAPI = city.New(*cityURL)
	}

	// ── 启动服务 ──
	srv := server.New(*dataDir, cityAPI, llmClient)
	srv.Resume()

	log.Fatal(srv.ListenAndServe(*port))
}
