package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type FeishuNotifier struct {
	webhookURL string
	secret     string
	client     *http.Client
}

func NewFeishuNotifier(webhookURL, secret string) *FeishuNotifier {
	return &FeishuNotifier{webhookURL: webhookURL, secret: secret, client: &http.Client{}}
}

// sign computes the Feishu webhook HMAC-SHA256 signature.
// Algorithm: base64(hmac_sha256(key=timestamp+"\n"+secret, message=""))
func (f *FeishuNotifier) sign(timestamp int64) string {
	stringToSign := strconv.FormatInt(timestamp, 10) + "\n" + f.secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (f *FeishuNotifier) Send(msg *NotifyMessage) error {
	if f.webhookURL == "" {
		return nil
	}

	var card map[string]interface{}
	if msg.Source == "cli" {
		card = f.buildCLICard(msg)
	} else {
		card = f.buildIdleCard(msg)
	}

	// Add signature if secret is configured
	if f.secret != "" {
		ts := time.Now().Unix()
		card["timestamp"] = strconv.FormatInt(ts, 10)
		card["sign"] = f.sign(ts)
	}

	body, err := json.Marshal(card)
	if err != nil {
		return err
	}
	resp, err := f.client.Post(f.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feishu webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// cliTypeConfig maps notification types to display properties.
var cliTypeConfig = map[string]struct {
	emoji    string
	title    string
	template string
}{
	"done":     {"✅", "任务完成", "green"},
	"confirm":  {"⏳", "需要确认", "orange"},
	"error":    {"❌", "出现错误", "red"},
	"progress": {"🔄", "任务进度", "blue"},
}

func (f *FeishuNotifier) buildCLICard(msg *NotifyMessage) map[string]interface{} {
	tc, ok := cliTypeConfig[msg.Type]
	if !ok {
		tc = cliTypeConfig["progress"]
	}

	elements := []interface{}{
		map[string]interface{}{
			"tag": "div",
			"text": map[string]string{
				"tag":     "lark_md",
				"content": fmt.Sprintf("%s %s", tc.emoji, msg.Message),
			},
		},
	}

	if msg.Context != "" {
		elements = append(elements,
			map[string]interface{}{"tag": "hr"},
			map[string]interface{}{
				"tag": "div",
				"text": map[string]string{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**详情:**\n%s", msg.Context),
				},
			},
		)
	}

	if msg.WebURL != "" {
		elements = append(elements, map[string]interface{}{
			"tag": "action",
			"actions": []interface{}{
				map[string]interface{}{
					"tag":  "button",
					"text": map[string]string{"tag": "plain_text", "content": "打开 Web 终端"},
					"url":  msg.WebURL,
					"type": "primary",
				},
			},
		})
	}

	return map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title":    map[string]string{"tag": "plain_text", "content": fmt.Sprintf("TermLive: %s", tc.title)},
				"template": tc.template,
			},
			"elements": elements,
		},
	}
}

func (f *FeishuNotifier) buildIdleCard(msg *NotifyMessage) map[string]interface{} {
	var headerTitle, headerTemplate string
	if msg.Confidence == "high" {
		headerTitle = fmt.Sprintf("TermLive: 终端等待输入 (空闲 %ds)", msg.IdleSeconds)
		headerTemplate = "red"
	} else {
		headerTitle = fmt.Sprintf("TermLive: 终端已空闲 %ds（可能仍在处理中）", msg.IdleSeconds)
		headerTemplate = "orange"
	}

	return map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title":    map[string]string{"tag": "plain_text", "content": headerTitle},
				"template": headerTemplate,
			},
			"elements": []interface{}{
				map[string]interface{}{
					"tag": "div",
					"text": map[string]string{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**会话:** %s (PID: %d)\n**运行时长:** %s", msg.Command, msg.Pid, msg.Duration),
					},
				},
				map[string]interface{}{
					"tag": "div",
					"text": map[string]string{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**最近输出:**\n```\n%s\n```", msg.LastOutput),
					},
				},
				map[string]interface{}{
					"tag": "action",
					"actions": []interface{}{
						map[string]interface{}{
							"tag":  "button",
							"text": map[string]string{"tag": "plain_text", "content": "打开 Web 终端"},
							"url":  msg.WebURL,
							"type": "primary",
						},
					},
				},
			},
		},
	}
}
