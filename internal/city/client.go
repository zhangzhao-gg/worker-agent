/**
 * [INPUT]: 依赖 net/http, math/rand, strings
 * [OUTPUT]: 对外提供 CityAPI struct 及全部城市交互方法（含 mock 模式）
 * [POS]: internal/city 的唯一成员，工人与外部世界的唯一 HTTP 接口
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package city

import (
	"math/rand"
	"net/http"
	"strings"
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

func (c *CityAPI) GetMyWorkAssignment(workerID string, occupation string) (string, error) {
	if c.mock {
		return mockWorkAssignment(occupation), nil
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
	"执政官令：所有可劳动居民明日起提前半小时到岗，优先保障煤炭产出",
	"通告：外环住房保温不足，工程队将在夜间进行紧急加固",
	"警告：暴风雪预警已升至二级，非必要人员不得离开城市边界",
	"通告：公共厨房今日供应热汤，儿童与病人优先领取",
	"执政官宣布：发现新的木材储备点，探索队将在天亮后出发确认",
	"通告：工坊需要志愿者搬运钢材，完成者可获得额外热饮配给",
	"警告：任何私藏煤炭的行为都会削弱整座城市的生存机会",
	"通告：医疗站开放夜间诊疗，咳嗽与发热者请尽快登记",
	"执政官令：为节省燃料，外环街灯将在午夜后熄灭",
	"好消息：猎人队带回充足肉食，今晚食堂将增加一份热汤",
	"通告：儿童庇护所需要更多看护者，识字者优先",
	"警告：冰层裂隙正在扩大，东侧采集路线暂时封闭",
	"执政官宣布：新法令将在礼拜后公布，请各工棚派代表到场",
	"通告：墓园区域冻土过硬，需要两组工人协助开凿",
	"好消息：工程师完成锅炉效率改良，预计今晚煤耗下降",
	"警告：伦敦帮活动频繁，居民不要轻信离城谣言",
	"通告：仓库将重新清点口粮，所有领取记录必须核验",
	"执政官令：紧急班次持续到风暴结束，缺勤者必须说明原因",
	"通告：南区水管冻结，取水点临时迁至锅炉广场",
	"警告：医疗站发现传染症状，探病时间即刻取消",
	"好消息：侦察员找到了旧世界仓库，可能含有罐头与工具",
	"通告：今日祈祷会将在晚班后举行，愿意参加者前往礼拜堂",
	"执政官宣布：生产榜将每日更新，贡献最高的小组优先获得补给",
	"警告：所有儿童不得靠近采煤机与锯木场，违者由监护人负责",
}

type workRule struct {
	Keywords   []string
	Assignment string
}

var mockWorkRules = []workRule{
	{[]string{"矿工", "采煤"}, "你今天的任务是在南矿区采煤。早八点到岗，晚六点收工。注意安全，服从工头调度。"},
	{[]string{"缝补", "裁缝", "工服"}, "你今天的任务是在锅炉房缝补工服。早八点到岗，晚六点收工。优先修补矿工破损的厚外套和手套，确保下井人员能御寒。"},
	{[]string{"锅炉"}, "你今天的任务是在锅炉房协助维护。早八点到岗，晚六点收工。听从锅炉工调度，确保供暖稳定。"},
	{[]string{"医生", "护士", "医护"}, "你今天的任务是在医疗站值守。早八点到岗，晚六点收工。处理冻伤、咳嗽和工伤，药品要节省使用。"},
	{[]string{"工程师", "技师", "机械"}, "你今天的任务是在工坊检修设备。早八点到岗，晚六点收工。优先处理影响供暖和采煤的机械故障。"},
	{[]string{"猎人", "侦察", "探险"}, "你今天的任务是外出侦察与搜集物资。早八点到岗，晚六点收工。结伴行动，注意狼群和暴风雪。"},
}

var mockNews = []string{
	"南矿区发生小规模塌方，三名工人被困，救援队正在清理塌落的冻土",
	"锅炉压力阀短暂失灵，工程师临时降压运行，外环住宅供暖明显减弱",
	"猎人队在白丘陵附近发现冻僵的难民队伍，其中还有两名儿童活着",
	"医疗站报告冻伤病例激增，医生请求优先分配木材加固病房墙体",
	"儿童庇护所的炉火昨夜熄灭，照看者请求增加一份煤炭配给",
	"探索队带回一枚完整的蒸汽核心，但护送雪橇在城门外损坏",
	"食堂宣布晚餐配给减半，居民队伍里出现了明显的骚动",
	"伦敦帮在礼拜堂外散发传单，声称旧世界仍有温暖和秩序",
	"秩序卫队逮捕了一名偷煤者，对方辩称家里的婴儿快冻死了",
	"工坊试制的新型采煤钻头断裂，碎片击伤了一名学徒",
	"墓园看守请求更多人手，昨夜新增的墓穴已经挖到冻土层以下",
	"温室玻璃被冰雹砸裂，农艺师担心幼苗在黎明前全部冻死",
	"煤堆旁发生争抢，两户家庭都声称那袋煤是自己的救命燃料",
	"侦察员在北方发现一座废弃观测站，里面可能还有可用地图",
	"一名矿工拒绝下井，声称昨夜听见矿坑深处传来求救声",
	"公共厨房的汤锅被发现掺了锯末，厨师说这是为了让所有人都能分到一碗",
	"锅炉旁的祈祷会持续到深夜，越来越多居民把希望寄托在火光上",
	"工程师建议延长工作班次，否则新住房无法在暴风雪前完工",
	"一支外出采集队失联超过六小时，最后信号来自裂冰湖附近",
	"监工报告有工人故意放慢进度，原因是加班后家人无人照看",
	"医疗站发现疑似肺炎扩散，医生要求立刻隔离咳血病人",
	"仓库清点发现罐头少了两箱，守卫怀疑是内部人员偷拿",
	"风暴前哨传回消息，气压正在急剧下降，三天内可能迎来白幕天气",
	"一名儿童在煤堆里找到旧时代的玩具火车，周围的人沉默了很久",
	"猎人队带回鹿肉，但其中一人承认他们在返程中丢下了受伤同伴",
	"临时宿舍的屋顶被积雪压弯，居民要求今晚转移到锅炉附近",
	"工坊完成了能量塔升级方案，但需要一枚蒸汽核心和大量钢材",
	"有居民在公告板上写下离城名单，已有十几个人悄悄签名",
	"医院病床已满，轻症病人被要求回家休养，引发家属抗议",
	"采煤机卡在冰层里，若强行启动可能损坏整套传动结构",
	"城市边缘发现狼群脚印，守卫建议禁止儿童独自外出",
	"一位老人把自己的晚餐让给邻居的孩子，随后在睡梦中停止呼吸",
	"信仰守护者宣布今晚举行赎罪仪式，希望平息居民的不满",
	"秩序宣传员要求各工棚张贴生产榜，落后小组将公开点名",
	"探索队找到一列被雪掩埋的货车，车厢里可能有木材和尸体",
	"锅炉燃煤效率突然下降，工程师怀疑煤里混入了过多冰渣",
	"一名母亲请求进入工坊工作，只为换取孩子的一张病床",
	"外环街道的灯塔熄灭，巡逻队在黑暗中迷路了近半小时",
	"居民会议要求废除童工，但煤炭库存只够维持两天",
	"城门口来了一个陌生人，他说南方还有一座没有熄火的城市",
}

func (c *CityAPI) mockHeartbeat() HeartbeatResponse {
	if rand.Intn(100) == 0 {
		return HeartbeatResponse{News: pickRandom(mockNews)}
	}
	return HeartbeatResponse{}
}

func mockWorkAssignment(occupation string) string {
	for _, rule := range mockWorkRules {
		if hasAny(occupation, rule.Keywords) {
			return rule.Assignment
		}
	}
	return "你今天的任务是在城市公共岗位轮值。早八点到岗，晚六点收工。服从调度，协助物资搬运、清扫积雪和临时修缮。"
}

func hasAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func pickRandom(options []string) string {
	return options[rand.Intn(len(options))]
}
