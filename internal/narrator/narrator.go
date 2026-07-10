/**
 * [INPUT]: 依赖 internal/db 持久化叙事决策，依赖 internal/llm 执行三工具叙事流程
 * [OUTPUT]: 对外提供 Narrator、Request、Outcome、CharacterProfile 和 Resolve()
 * [POS]: internal/narrator 的系统级 agent，不参与普通 worker 心跳，只负责联系人缺口的世界补全
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package narrator

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"worker-agent/internal/db"
	"worker-agent/internal/llm"
)

const (
	Ask    = "ask"
	Create = "create"
	Reject = "reject"
)

type Request struct {
	Requester        string
	Target           string
	Message          string
	RequesterSoul    db.Soul
	RequesterMemory  []db.Memory
	RequesterContact []db.Contact
}

type Outcome struct {
	Kind      string
	Message   string
	Question  string
	Reason    string
	Character CharacterProfile
}

type CharacterProfile struct {
	Name        string `json:"name"`
	Occupation  string `json:"occupation"`
	Background  string `json:"background"`
	Personality string `json:"personality"`
	SpeechStyle string `json:"speech_style"`
	ValuesDesc  string `json:"values_desc"`
	Family      string `json:"family"`
	Relation    string `json:"relation"`
	Notes       string `json:"notes"`
}

type StartFunc func(CharacterProfile) error

type Narrator struct {
	db    *db.Database
	llm   llm.Client
	start StartFunc
	log   func(int, string, string)
}

func New(database *db.Database, client llm.Client, start StartFunc) *Narrator {
	return &Narrator{db: database, llm: client, start: start}
}

func (n *Narrator) Resolve(req Request) Outcome {
	sessionID := "narrator_" + time.Now().Format("20060102_150405")
	n.log = func(round int, typ string, content string) {
		_ = n.db.InsertReasoningLog(sessionID, round, typ, content)
	}
	log.Printf("[narrator] 收到联系人缺口: requester=%s target=%s message=%q", req.Requester, req.Target, req.Message)

	if n.llm == nil {
		return n.record(req, Outcome{Kind: Ask, Question: askQuestion(req.Target), Reason: "LLM 未初始化"})
	}

	prompt := systemPrompt()
	input := requestText(req)
	n.log(0, "system_prompt", prompt)
	n.log(0, "input", input)

	resp, err := n.llm.Chat(prompt, []llm.Message{{Role: "user", Content: input}}, tools())
	if err != nil {
		log.Printf("[narrator] LLM 调用失败: target=%s err=%v", req.Target, err)
		return n.record(req, Outcome{Kind: Ask, Question: askQuestion(req.Target), Reason: err.Error()})
	}
	if resp.Message.Content != "" {
		n.log(1, "llm_text", resp.Message.Content)
	}

	out, started := n.runTools(req, resp.Message.ToolCalls)
	if out.Kind == "" {
		out = Outcome{Kind: Ask, Question: askQuestion(req.Target)}
	}
	if out.Kind == Create && !started {
		out = n.startCreated(req, out)
	}
	return n.record(req, out)
}

func (n *Narrator) runTools(req Request, calls []llm.ToolCall) (Outcome, bool) {
	var out Outcome
	started := false

	for _, call := range calls {
		log.Printf("[narrator] tool: %s(%s)", call.Function.Name, call.Function.Arguments)
		n.log(1, "tool_call", call.Function.Name+"("+call.Function.Arguments+")")
		args := parseArgs(call.Function.Arguments)

		switch call.Function.Name {
		case "decide_character":
			out = applyDecision(req, out, args)
		case "write_character":
			out.Kind = Create
			out.Character = characterFromArgs(req.Target, args)
			n.log(1, "tool_result", "character written")
		case "start_agent":
			if out.Character.Name == "" {
				out.Character.Name = textArg(args, "name")
			}
			out = n.startCreated(req, out)
			started = out.Kind == Create
			n.log(1, "tool_result", out.Message)
		}
	}
	return out, started
}

func applyDecision(req Request, out Outcome, args map[string]any) Outcome {
	action := textArg(args, "action")
	switch action {
	case Ask:
		out.Kind = Ask
		out.Question = textArg(args, "question")
		if out.Question == "" {
			out.Question = askQuestion(req.Target)
		}
	case Create:
		out.Kind = Create
		out.Reason = textArg(args, "reason")
	case Reject:
		out.Kind = Reject
		out.Reason = textArg(args, "reason")
		if out.Reason == "" {
			out.Reason = "信息不足，无法确认其在世界中的位置"
		}
	}
	return out
}

func (n *Narrator) startCreated(req Request, out Outcome) Outcome {
	if out.Character.Name == "" {
		out.Kind = Ask
		out.Question = askQuestion(req.Target)
		return out
	}
	if err := n.start(out.Character); err != nil {
		return Outcome{Kind: Ask, Question: askQuestion(req.Target), Reason: err.Error()}
	}
	out.Kind = Create
	out.Message = fmt.Sprintf("叙事者让「%s」进入了新伦敦。你现在可以再次联系他。", out.Character.Name)
	return out
}

func (n *Narrator) record(req Request, out Outcome) Outcome {
	if out.Message == "" {
		out.Message = message(req, out)
	}
	log.Printf("[narrator] 裁定完成: target=%s kind=%s worker=%s reason=%s question=%s", req.Target, out.Kind, out.Character.Name, out.Reason, out.Question)
	_ = n.db.InsertNarratorDecision(db.NarratorDecision{
		Requester:        req.Requester,
		RequestedName:    req.Target,
		RequestedMessage: req.Message,
		Decision:         out.Kind,
		Reason:           first(out.Reason, out.Question),
		CreatedWorker:    out.Character.Name,
	})
	return out
}

func message(req Request, out Outcome) string {
	switch out.Kind {
	case Create:
		return fmt.Sprintf("叙事者让「%s」进入了新伦敦。你现在可以再次联系他。", first(out.Character.Name, req.Target))
	case Reject:
		return fmt.Sprintf("叙事者拒绝让「%s」成为可联系角色：%s", req.Target, out.Reason)
	default:
		return "叙事者需要更多信息：" + first(out.Question, askQuestion(req.Target))
	}
}

func systemPrompt() string {
	return `你是「叙事者」，新伦敦世界的系统级编剧，只负责联系人缺口的世界补全。

工具顺序必须清晰：
1. decide_character 判断 ask/create/reject。
2. create 时必须 write_character 写完整角色。
3. write_character 后必须 start_agent。

规则：
- 信息不足、名字像称谓或无法落位，ask。
- 关系和世界位置足够清楚，create。
- 破坏世界观、现代乱入或恶意构造，reject。
- 创建角色必须符合新伦敦语境。`
}

func requestText(req Request) string {
	contacts, _ := json.Marshal(req.RequesterContact)
	memories, _ := json.Marshal(req.RequesterMemory)
	return fmt.Sprintf(`请求者：%s
身份：%s，%s
背景：%s
想联系：%s
原始消息：%s

联系人：%s
近期记忆：%s`, req.Requester, req.RequesterSoul.Name, req.RequesterSoul.Occupation, req.RequesterSoul.Background, req.Target, req.Message, contacts, memories)
}

func tools() []llm.ToolDef {
	raw := `[
  {"name":"decide_character","description":"判断是否问询、创建或拒绝让一个联系人进入世界。","input_schema":{"type":"object","properties":{"action":{"type":"string","enum":["ask","create","reject"]},"question":{"type":"string"},"reason":{"type":"string"}},"required":["action"]}},
  {"name":"write_character","description":"写入即将创建的角色完整资料。","input_schema":{"type":"object","properties":{"name":{"type":"string"},"occupation":{"type":"string"},"background":{"type":"string"},"personality":{"type":"string"},"speech_style":{"type":"string"},"values_desc":{"type":"string"},"family":{"type":"string"},"relation":{"type":"string"},"notes":{"type":"string"}},"required":["name","occupation","background","personality","speech_style","values_desc","family","relation"]}},
  {"name":"start_agent","description":"启动已经写好资料的新角色 agent。","input_schema":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}}
]`
	var defs []llm.ToolDef
	_ = json.Unmarshal([]byte(raw), &defs)
	return defs
}

func parseArgs(raw string) map[string]any {
	var args map[string]any
	_ = json.Unmarshal([]byte(raw), &args)
	return args
}

func characterFromArgs(defaultName string, args map[string]any) CharacterProfile {
	return CharacterProfile{
		Name:        first(textArg(args, "name"), defaultName),
		Occupation:  textArg(args, "occupation"),
		Background:  textArg(args, "background"),
		Personality: textArg(args, "personality"),
		SpeechStyle: textArg(args, "speech_style"),
		ValuesDesc:  textArg(args, "values_desc"),
		Family:      textArg(args, "family"),
		Relation:    textArg(args, "relation"),
		Notes:       textArg(args, "notes"),
	}
}

func textArg(args map[string]any, key string) string {
	s, _ := args[key].(string)
	return strings.TrimSpace(s)
}

func askQuestion(name string) string {
	return fmt.Sprintf("你说的「%s」是谁？他与你是什么关系？你为什么现在要联系他？", name)
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
