/**
 * [INPUT]: 依赖 internal/server, internal/city, internal/llm
 * [OUTPUT]: 对外提供 worker-agent 可执行入口
 * [POS]: 进程启动点，初始化依赖，启动 HTTP 服务 + 自动恢复
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"

	"worker-agent/internal/city"
	"worker-agent/internal/llm"
	"worker-agent/internal/server"
)

func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func main() {
	loadEnv(".env")

	port := flag.Int("port", 8080, "HTTP 服务端口")
	dataDir := flag.String("data", "./data", "数据目录（存放工人 DB 文件）")
	cityURL := flag.String("city", "", "城市 API 地址（空则启用 mock 模式）")
	llmURL := flag.String("llm-url", "https://api.minimaxi.com", "LLM API 地址")
	llmKey := flag.String("llm-key", "", "LLM API Key（默认读 LLM_API_KEY 环境变量）")
	llmModel := flag.String("llm-model", "MiniMax-M2.7", "LLM 模型名称")
	flag.Parse()

	// ── LLM 客户端 ──
	apiKey := *llmKey
	if apiKey == "" {
		apiKey = os.Getenv("LLM_API_KEY")
	}
	var llmClient llm.Client
	if apiKey == "" {
		log.Println("[main] LLM API Key 未配置，工人推理功能不可用（UI 和 API 正常）")
	} else {
		llmClient = llm.NewMiniMax(*llmURL, apiKey, *llmModel)
	}

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
