package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeChatNotifyHighConfidence(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	n := NewWeChatNotifier(server.URL)
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
	if receivedBody["msgtype"] != "markdown" {
		t.Errorf("expected msgtype 'markdown', got %v", receivedBody["msgtype"])
	}
	md := receivedBody["markdown"].(map[string]interface{})
	content := md["content"].(string)
	if !strings.Contains(content, "终端等待输入") {
		t.Errorf("high confidence should contain '终端等待输入', got %s", content)
	}
}

func TestWeChatNotifyLowConfidence(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	n := NewWeChatNotifier(server.URL)
	msg := &NotifyMessage{
		SessionID:   "abc123",
		Command:     "claude",
		Pid:         12345,
		Duration:    "15m 32s",
		LastOutput:  "? Do you want to proceed? [Y/n]",
		WebURL:      "http://192.168.1.5:8080/s/abc123",
		IdleSeconds: 30,
		Confidence:  "low",
	}
	err := n.Send(msg)
	if err != nil {
		t.Fatal(err)
	}
	if receivedBody == nil {
		t.Fatal("expected request body")
	}
	md := receivedBody["markdown"].(map[string]interface{})
	content := md["content"].(string)
	if !strings.Contains(content, "可能仍在处理中") {
		t.Errorf("low confidence should contain '可能仍在处理中', got %s", content)
	}
}

func TestWeChatNotifyEmptyURL(t *testing.T) {
	n := NewWeChatNotifier("")
	err := n.Send(&NotifyMessage{})
	if err != nil {
		t.Error("empty URL should be a no-op, not an error")
	}
}
