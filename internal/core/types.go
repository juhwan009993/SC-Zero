package core

import "time"

// 대화 내용 구조체
type ChatMessage struct {
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// 상수 정의
const (
	ProtocolID  = "/sc-zero/1.0.0"
	KeyFile     = "identity.key"
	HistoryFile = "chat_history.enc"
)