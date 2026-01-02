package core

import "time"

const (
	ProtocolID = "/sc-zero/2.0.0"
	Difficulty = 3 // PoW 난이도 (0의 개수, 높을수록 오래 걸림)
)

// NetworkPacket: P2P로 주고받는 데이터의 포장지
type NetworkPacket struct {
	Type    string `json:"type"`    // AUTH, MSG, GOSSIP, REQUEST, RESPONSE
	Payload string `json:"payload"` // JSON Data
}

// ChatMessage: 대화 내용 구조체
type ChatMessage struct {
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}