package mem

import (
	"fmt"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func newTestMessage(role schema.RoleType, content string) *schema.Message {
	return &schema.Message{Role: role, Content: content}
}

func contentsOf(msgs []*schema.Message) []string {
	contents := make([]string, 0, len(msgs))
	for _, m := range msgs {
		contents = append(contents, m.Content)
	}
	return contents
}

// TestGetSimpleMemory_CreatesAndCaches 同一会话 ID 返回同一实例，不同会话相互隔离
func TestGetSimpleMemory_CreatesAndCaches(t *testing.T) {
	m1 := GetSimpleMemory("session-1")
	m2 := GetSimpleMemory("session-1")
	if m1 != m2 {
		t.Fatalf("GetSimpleMemory with the same id should return the same instance")
	}
	if m1.MaxWindowSize != 6 {
		t.Fatalf("default MaxWindowSize should be 6, got %d", m1.MaxWindowSize)
	}
	if msgs := m1.GetMessages(); len(msgs) != 0 {
		t.Fatalf("a new session should have empty history, got %d messages", len(msgs))
	}

	m3 := GetSimpleMemory("session-2")
	if m1 == m3 {
		t.Fatalf("different session ids should return different instances")
	}
}

// TestSetMessages_KeepsHistoryWithinWindow 消息数不超过窗口大小时不裁剪
func TestSetMessages_KeepsHistoryWithinWindow(t *testing.T) {
	mem := GetSimpleMemory("window-keep")
	for i := 0; i < 6; i++ {
		mem.SetMessages(newTestMessage(schema.User, fmt.Sprintf("msg-%d", i)))
	}
	if msgs := mem.GetMessages(); len(msgs) != 6 {
		t.Fatalf("expected 6 messages within the window, got %d", len(msgs))
	}
}

// TestSetMessages_SlidingWindow_TrimsOldest 超出窗口后丢弃最早的偶数条消息
func TestSetMessages_SlidingWindow_TrimsOldest(t *testing.T) {
	mem := GetSimpleMemory("window-trim")
	for i := 0; i < 7; i++ {
		mem.SetMessages(newTestMessage(schema.User, fmt.Sprintf("msg-%d", i)))
	}
	// 7 条超出上限 1 条（奇数），补齐为偶数 2，丢弃最早的 2 条，保留 5 条
	msgs := mem.GetMessages()
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages after trimming 2, got %d", len(msgs))
	}
	if msgs[0].Content != "msg-2" {
		t.Fatalf("oldest messages should be trimmed first, first message = %q", msgs[0].Content)
	}
	if msgs[len(msgs)-1].Content != "msg-6" {
		t.Fatalf("latest message should be kept, last message = %q", msgs[len(msgs)-1].Content)
	}

	// 第 8 条加入后恢复为 6 条
	mem.SetMessages(newTestMessage(schema.User, "msg-7"))
	if msgs := mem.GetMessages(); len(msgs) != 6 {
		t.Fatalf("expected 6 messages after adding one back, got %d", len(msgs))
	}
}

// TestSetMessages_TrimsInPairs 裁剪数量恒为偶数，保证窗口以用户消息开头（用户/AI成对对齐）
func TestSetMessages_TrimsInPairs(t *testing.T) {
	mem := GetSimpleMemory("window-pairs")
	for i := 0; i < 4; i++ {
		mem.SetMessages(newTestMessage(schema.User, fmt.Sprintf("U%d", i)))
		mem.SetMessages(newTestMessage(schema.Assistant, fmt.Sprintf("A%d", i)))
	}
	// 8 条超出 6 条：excess=2（偶数），丢弃最早的一对 U0/A0，保留 U1A1 U2A2 U3A3
	msgs := mem.GetMessages()
	if len(msgs) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(msgs))
	}
	if msgs[0].Role != schema.User {
		t.Fatalf("window should start with a user message, got role %q", msgs[0].Role)
	}
	want := []string{"U1", "A1", "U2", "A2", "U3", "A3"}
	if got := contentsOf(msgs); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("unexpected window contents: %v, want %v", got, want)
	}
}

// TestGetMessages_ReturnsSnapshot 返回值应是快照副本：调用方修改返回切片不得影响内部历史
func TestGetMessages_ReturnsSnapshot(t *testing.T) {
	mem := GetSimpleMemory("snapshot")
	mem.SetMessages(newTestMessage(schema.User, "original"))

	got := mem.GetMessages()
	got[0].Content = "mutated"
	got = append(got, newTestMessage(schema.Assistant, "injected"))

	inner := mem.GetMessages()
	if inner[0].Content != "original" {
		t.Fatalf("mutating the returned slice leaked into stored history: %q", inner[0].Content)
	}
	if len(inner) != 1 {
		t.Fatalf("appending to the returned slice changed stored history length: %d", len(inner))
	}
}
