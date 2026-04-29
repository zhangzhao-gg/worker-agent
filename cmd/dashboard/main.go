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

	dash := &dashboard{
		dataDir: *dataDir,
		conns:   make(map[string]*db.Database),
	}

	mux := http.NewServeMux()
	webHandler := web.New(dash.entries)
	webHandler.Register(mux)

	avatarDir := filepath.Join(*dataDir, "avatars")
	os.MkdirAll(avatarDir, 0755)
	mux.Handle("GET /static/avatars/", http.StripPrefix("/static/avatars/", http.FileServer(http.Dir(avatarDir))))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[dashboard] 启动: %s  数据目录: %s", addr, *dataDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// ================================================================
//  长连接池，每次请求刷新文件列表
// ================================================================

type dashboard struct {
	dataDir string
	mu      sync.Mutex
	conns   map[string]*db.Database
}

func (d *dashboard) entries() []web.WorkerEntry {
	dir, err := os.ReadDir(d.dataDir)
	if err != nil {
		log.Printf("[dashboard] 扫描目录失败: %v", err)
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// 收集当前有效的 db 文件
	activeFiles := make(map[string]bool)
	for _, f := range dir {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".db") {
			continue
		}
		name := strings.TrimSuffix(f.Name(), ".db")
		activeFiles[name] = true

		if _, exists := d.conns[name]; !exists {
			dbPath := filepath.Join(d.dataDir, f.Name())
			database, err := db.NewReadOnly(dbPath)
			if err != nil {
				log.Printf("[dashboard] 打开 %s 失败: %v", name, err)
				continue
			}
			d.conns[name] = database
		}
	}

	// 清理已删除的 db
	for name, conn := range d.conns {
		if !activeFiles[name] {
			conn.Close()
			delete(d.conns, name)
		}
	}

	// 组装结果
	var entries []web.WorkerEntry
	for name, database := range d.conns {
		if _, err := database.GetSoul(); err != nil {
			continue
		}

		status := "stopped"
		if hasPending, _ := database.HasPendingWakeups(); hasPending {
			status = "running"
		}

		entries = append(entries, web.WorkerEntry{
			Name:   name,
			Status: status,
			DB:     database,
		})
	}

	return entries
}
