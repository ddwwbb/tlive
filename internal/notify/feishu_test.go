package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeishuNotifyHighConfidence(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer server.Close()

	n := NewFeishuNotifier(server.URL)
	msg := &NotifyMessage{
		SessionID:   "abc123",
		Command:     "claude",
		Pid:         12345,
		Duration:    "15m 32s",
		LastOutput:  "? Do you want to proceed? [Y/n]",
		WebURL:      "http://192.168.1.5:8080/s/abc123",
		IdleSeconds: 30,
		Confidence:  "high",
	}
	err := n.Send(msg)
	if err != nil {
		t.Fatal(err)
	}
	if receivedBody == nil {
		t.Fatal("expected request body")
	}
	if receivedBody["msg_type"] != "interactive" {
		t.Errorf("expected msg_type 'interactive', got %v", receivedBody["msg_type"])
	}
	card := receivedBody["card"].(map[string]interface{})
	header := card["header"].(map[string]interface{})
	title := header["title"].(map[string]interface{})
	content := title["content"].(string)
	if content != "TermLive: 终端等待输入 (空闲 30s)" {
		t.Errorf("high confidence title mismatch, got %s", content)
	}
	if header["template"] != "red" {
		t.Errorf("high confidence template should be 'red', got %v", header["template"])
	}
}

func TestFeishuNotifyLowConfidence(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer server.Close()

	n := NewFeishuNotifier(server.URL)
	msg := &NotifyMessage{
		SessionID:   "abc123",
		Command:     "claude",
		Pid:         12345,
		Duration:    "15m 32s",
		LastOutput:  "Building project...",
		WebURL:      "http://192.168.1.5:8080/s/abc123",
		IdleSeconds: 60,
		Confidence:  "low",
	}
	err := n.Send(msg)
	if err != nil {
		t.Fatal(err)
	}
	if receivedBody == nil {
		t.Fatal("expected request body")
	}
	card := receivedBody["card"].(map[string]interface{})
	header := card["header"].(map[string]interface{})
	title := header["title"].(map[string]interface{})
	content := title["content"].(string)
	if content != "TermLive: 终端已空闲 60s（可能仍在处理中）" {
		t.Errorf("low confidence title mismatch, got %s", content)
	}
	if header["template"] != "orange" {
		t.Errorf("low confidence template should be 'orange', got %v", header["template"])
	}
}

func TestFeishuNotifyEmptyURL(t *testing.T) {
	n := NewFeishuNotifier("")
	err := n.Send(&NotifyMessage{})
	if err != nil {
		t.Error("empty URL should be a no-op, not an error")
	}
}
