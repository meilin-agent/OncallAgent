package mem

import (
	"sync"

	"github.com/cloudwego/eino/schema"
)

var SimpleMemoryMap = make(map[string]*SimpleMemory)
var mu sync.Mutex

// GetSimpleMemory 返回指定会话ID的内存对象，不存在则创建并缓存
func GetSimpleMemory(id string) *SimpleMemory {
	mu.Lock()
	defer mu.Unlock()
	// 如果已存在会话内存则直接返回，否则新建一个并保存
	if mem, ok := SimpleMemoryMap[id]; ok {
		return mem
	} else {
		newMem := &SimpleMemory{
			ID:            id,
			Messages:      []*schema.Message{},
			MaxWindowSize: 6,
		}
		SimpleMemoryMap[id] = newMem
		return newMem
	}
}

type SimpleMemory struct {
	ID            string            `json:"id"`
	Messages      []*schema.Message `json:"messages"`
	MaxWindowSize int
	mu            sync.Mutex
}

// SetMessages 将消息追加到当前会话历史，并对超出长度进行滑动窗口裁剪
func (c *SimpleMemory) SetMessages(msg *schema.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Messages = append(c.Messages, msg)
	if len(c.Messages) > c.MaxWindowSize {
		// 确保成对丢弃消息，保持用户/AI消息对齐
		// 计算需要丢弃的消息数量（必须是偶数）
		excess := len(c.Messages) - c.MaxWindowSize
		if excess%2 != 0 {
			excess++ // 确保丢弃偶数条消息
		}
		// 丢弃最早的消息，保留最近上下文
		c.Messages = c.Messages[excess:]
	}
}

// GetMessages 返回当前会话的历史消息（快照副本）
func (c *SimpleMemory) GetMessages() []*schema.Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 返回副本而非内部切片别名，避免调用方通过返回值修改内部存储，
	// 同时防止锁释放后对共享底层数组的读写形成数据竞争
	msgs := make([]*schema.Message, len(c.Messages))
	for i, m := range c.Messages {
		msgCopy := *m
		msgs[i] = &msgCopy
	}
	return msgs
}
