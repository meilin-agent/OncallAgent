package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestGetCurrentTimeTool_Name 工具名称符合 LLM 意图识别约定
func TestGetCurrentTimeTool_Name(t *testing.T) {
	info, err := NewGetCurrentTimeTool().Info(context.Background())
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	if info.Name != "get_current_time" {
		t.Fatalf("unexpected tool name: %q", info.Name)
	}
}

// TestGetCurrentTimeTool_Invoke 调用返回的各时间字段自洽且接近当前时间
func TestGetCurrentTimeTool_Invoke(t *testing.T) {
	ctx := context.Background()
	tool := NewGetCurrentTimeTool()

	before := time.Now().Unix()
	out, err := tool.InvokableRun(ctx, "{}")
	after := time.Now().Unix()
	if err != nil {
		t.Fatalf("InvokableRun failed: %v", err)
	}

	var result GetCurrentTimeOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, out)
	}
	if !result.Success {
		t.Fatalf("expected success=true")
	}
	// 秒/毫秒/微秒三级时间戳应自洽
	if result.Seconds != result.Milliseconds/1000 {
		t.Fatalf("seconds %d inconsistent with milliseconds %d", result.Seconds, result.Milliseconds)
	}
	if result.Milliseconds != result.Microseconds/1000 {
		t.Fatalf("milliseconds %d inconsistent with microseconds %d", result.Milliseconds, result.Microseconds)
	}
	// 返回时间应落在调用前后（含 1 秒容差）
	if result.Seconds < before-1 || result.Seconds > after+1 {
		t.Fatalf("returned time %d out of range [%d, %d]", result.Seconds, before, after)
	}
	// 人类可读时间戳可解析，且与 Unix 秒级时间一致（按本地时区解析）
	ts, err := time.ParseInLocation("2006-01-02 15:04:05.000000", result.Timestamp, time.Local)
	if err != nil {
		t.Fatalf("timestamp %q is not parseable: %v", result.Timestamp, err)
	}
	if ts.Unix() != result.Seconds {
		t.Fatalf("timestamp %q does not match unix seconds %d", result.Timestamp, result.Seconds)
	}
}
