package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type FeishuNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewFeishuNotifier(webhookURL string) *FeishuNotifier {
	return &FeishuNotifier{webhookURL: webhookURL, client: &http.Client{}}
}

func (f *FeishuNotifier) Send(msg *NotifyMessage) error {
	if f.webhookURL == "" {
		return nil
	}
	var headerTitle string
	var headerTemplate string
	if msg.Confidence == "high" {
		headerTitle = fmt.Sprintf("TermLive: 终端等待输入 (空闲 %ds)", msg.IdleSeconds)
		headerTemplate = "red"
	} else {
		headerTitle = fmt.Sprintf("TermLive: 终端已空闲 %ds（可能仍在处理中）", msg.IdleSeconds)
		headerTemplate = "orange"
	}
	card := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"tag":     "plain_text",
					"content": headerTitle,
				},
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
