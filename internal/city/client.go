/**
 * [INPUT]: 依赖 net/http, encoding/json, math/rand
 * [OUTPUT]: 对外提供 CityAPI struct 及全部城市交互方法（含 mock 模式）
 * [POS]: internal/city 的唯一成员，工人与外部世界的唯一 HTTP 接口
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package city

import (
	"math/rand"
	"net/http"
	"time"
)

// ================================================================
//  核心结构体
// ================================================================

type CityAPI struct {
	baseURL string
	client  *http.Client
	mock    bool
}

type HeartbeatResponse struct {
	News string `json:"news"`
}

func New(baseURL string) *CityAPI {
	return &CityAPI{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		mock:    false,
	}
}

func NewMock() *CityAPI {
	return &CityAPI{mock: true}
}

// ================================================================
//  心跳
// ================================================================

func (c *CityAPI) Heartbeat(workerID string) (HeartbeatResponse, error) {
	if c.mock {
		return c.mockHeartbeat(), nil
	}
	// TODO: 实现 HTTP 调用
	return HeartbeatResponse{}, nil
}

// ================================================================
//  叙事同步
// ================================================================

func (c *CityAPI) PostNarrative(workerID string, text string) error {
	if c.mock {
		return nil
	}
	// TODO: 实现 HTTP 调用
	return nil
}

// ================================================================
//  感知接口（供推理引擎 tools 调用）
// ================================================================

func (c *CityAPI) GetCityTemperature() (string, error) {
	if c.mock {
		day := time.Now().YearDay()
		return mockTemperatures[day%len(mockTemperatures)], nil
	}
	// TODO: 实现 HTTP 调用
	return "", nil
}

func (c *CityAPI) GetFoodStatus() (string, error) {
	if c.mock {
		day := time.Now().YearDay()
		return mockFoodStatus[day%len(mockFoodStatus)], nil
	}
	// TODO: 实现 HTTP 调用
	return "", nil
}

func (c *CityAPI) GetCityAnnouncements() ([]string, error) {
	if c.mock {
		day := time.Now().YearDay()
		return []string{mockAnnouncements[day%len(mockAnnouncements)]}, nil
	}
	// TODO: 实现 HTTP 调用
	return nil, nil
}

func (c *CityAPI) GetMyWorkAssignment(workerID string) (string, error) {
	if c.mock {
		return "你今天的任务是在南矿区采煤。早八点到岗，晚六点收工。注意安全，服从工头调度。", nil
	}
	// TODO: 实现 HTTP 调用
	return "", nil
}

// ================================================================
//  Mock 数据
// ================================================================

var mockTemperatures = []string{
	"寒冷刺骨，锅炉在全力运转，但仍然不够暖和",
	"气温略有回升，但远称不上舒适",
	"刺骨的寒风从北方吹来，积雪越来越厚",
	"今天比昨天暖和一些，锅炉的煤耗降低了",
	"暴风雪即将来临，空气中弥漫着冰冷的水汽",
}

var mockFoodStatus = []string{
	"配给紧张，排队的人越来越多",
	"食物供应正常，但品种单调",
	"配给充裕，今天有额外的罐头分发",
	"食物储备在减少，可能很快会削减配给",
	"听说猎人队带回了一批鹿肉，食堂今天会有改善",
}

var mockAnnouncements = []string{
	"执政官宣布：锅炉维修工作将在本周完成，届时供暖将改善",
	"通告：南区的新住房即将完工，请有需要的工人前往登记",
	"警告：近期有狼群出没，外出作业请结伴而行",
	"好消息：探索队在东边发现了新的煤矿脉",
	"执政官令：为应对寒潮，今日起加班一小时",
	"通告：医疗站缺少药品，请有多余草药的居民捐献",
}

var mockAssignments = []string{
	"今天的任务是在南矿区采煤，注意安全",
	"你被分配到锅炉房维护工作，确保供暖正常",
	"今天去伐木场帮忙，暴风雪前需要储备木材",
	"被安排到建筑工地，新住房需要更多人手",
	"今天的任务是在仓库整理物资，清点库存",
}

var mockNews = []string{
	"",
	"",
	"",
	"",
	"",
	"南矿区发生小规模塌方，暂无人员伤亡",
	"一名工人在暴风雪中走失，搜救队正在出动",
	"探索队归来，带回了珍贵的蒸汽核心",
}

func (c *CityAPI) mockHeartbeat() HeartbeatResponse {
	if rand.Intn(100) == 0 {
		return HeartbeatResponse{News: pickRandom(mockNews[5:])}
	}
	return HeartbeatResponse{}
}

func pickRandom(options []string) string {
	return options[rand.Intn(len(options))]
}
