package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sc-zero/internal/core"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
)

type IdentityBlock struct {
	Timestamp int64  `json:"timestamp"`
	PeerID    string `json:"peer_id"`
	PubKey    []byte `json:"pub_key"`   // 검증용 공개키
	Signature []byte `json:"signature"` // 개인키 서명
	Hash      string `json:"hash"`      // PoW 결과
	Nonce     int    `json:"nonce"`     // PoW 노력값
}

// CreateIdentityBlock: 채굴을 포함한 블록 생성
func CreateIdentityBlock(priv crypto.PrivKey, peerID string) (*IdentityBlock, error) {
	pubBytes, err := crypto.MarshalPublicKey(priv.GetPublic())
	if err != nil {
		return nil, err
	}

	block := &IdentityBlock{
		Timestamp: time.Now().Unix(),
		PeerID:    peerID,
		PubKey:    pubBytes,
		Nonce:     0,
	}

	// 1. 서명 (데이터 무결성 보장)
	signData := fmt.Sprintf("%s%d", block.PeerID, block.Timestamp)
	sig, err := priv.Sign([]byte(signData))
	if err != nil {
		return nil, err
	}
	block.Signature = sig

	// 2. 채굴 (PoW)
	block.Mine()

	return block, nil
}

func (b *IdentityBlock) CalculateHash() string {
	record := fmt.Sprintf("%d%s%x%x%d", b.Timestamp, b.PeerID, b.PubKey, b.Signature, b.Nonce)
	h := sha256.New()
	h.Write([]byte(record))
	return hex.EncodeToString(h.Sum(nil))
}

func (b *IdentityBlock) Mine() {
	target := strings.Repeat("0", core.Difficulty)
	for {
		b.Hash = b.CalculateHash()
		if strings.HasPrefix(b.Hash, target) {
			break
		}
		b.Nonce++
	}
}

func (b *IdentityBlock) ToJSON() string {
	data, _ := json.Marshal(b)
	return string(data)
}

func FromJSON(data string) (*IdentityBlock, error) {
	var b IdentityBlock
	err := json.Unmarshal([]byte(data), &b)
	return &b, err
}