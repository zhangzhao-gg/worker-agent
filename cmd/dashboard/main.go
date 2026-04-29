/**
 * [INPUT]: 依赖 internal/db, internal/web
 * [OUTPUT]: 对外提供 dashboard 独立可执行入口
 * [POS]: 独立于 worker 进程的只读 UI，扫描 data/*.db 渲染 Web 界面
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"worker-agent/internal/db"
	"worker-agent/internal/web"
)

func main() {
	port := flag.Int("port", 8081, "Dashboard HTTP 端口")
	dataDir := flag.String("data", "./data", "数据目录（与 worker 共享）")
	flag.Parse()

	dash := &dashboard{dataDir: *dataDir}

	mux := http.NewServeMux()
	webHandler := web.New(dash.entries)
	webHandler.Register(mux)

	// 静态文件（头像）
	avatarDir := filepath.Join(*dataDir, "avatars")
	os.MkdirAll(avatarDir, 0755)
	mux.Handle("GET /static/avatars/", http.StripPrefix("/static/avatars/", http.FileServer(http.Dir(avatarDir))))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[dashboard] 启动: %s  数据目录: %s", addr, *dataDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// ================================================================
//  每次请求实时扫描 DB 文件，无缓存，始终反映最新状态
// ================================================================

type dashboard struct {
	dataDir string
}

func (d *dashboard) entries() []web.WorkerEntry {
	dir, err := os.ReadDir(d.dataDir)
	if err != nil {
		log.Printf("[dashboard] 扫描目录失败: %v", err)
		return nil
	}

	var (
		mu      sync.Mutex
		entries []web.WorkerEntry
		wg      sync.WaitGroup
	)

	for _, f := range dir {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".db") {
			continue
		}

		name := strings.TrimSuffix(f.Name(), ".db")
		dbPath := filepath.Join(d.dataDir, f.Name())

		wg.Add(1)
		go func(name, dbPath string) {
			defer wg.Done()

			database, err := db.NewReadOnly(dbPath)
			if err != nil {
				log.Printf("[dashboard] 打开 %s 失败: %v", name, err)
				return
			}
			defer database.Close()

			if _, err := database.GetSoul(); err != nil {
				return
			}

			mu.Lock()
			entries = append(entries, web.WorkerEntry{
				Name:   name,
				Status: "unknown",
				DB:     database,
			})
			mu.Unlock()
		}(name, dbPath)
	}

	wg.Wait()
	return entries
}
