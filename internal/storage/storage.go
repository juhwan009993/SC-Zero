package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sc-zero/internal/blockchain"
	"sc-zero/internal/core"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
)

const (
	KeyFile      = "identity.key"
	IdentityFile = "my_identity.enc"
	HistoryFile  = "chat_history.enc"
)

// 전역 변수로 메모리상 대화 기록 관리
var ChatHistory []core.ChatMessage

// --- Chat History Management ---

func AppendLog(sender, content string) {
	msg := core.ChatMessage{
		Sender:    sender,
		Content:   content,
		Timestamp: time.Now(),
	}
	ChatHistory = append(ChatHistory, msg)
}

func GetHistory() []core.ChatMessage {
	return ChatHistory
}

// --- 키 관리 ---

func LoadOrGenerateKey() (crypto.PrivKey, error) {
	if _, err := os.Stat(KeyFile); err == nil {
		data, err := os.ReadFile(KeyFile)
		if err != nil { return nil, err }
		keyBytes, err := hex.DecodeString(string(data))
		if err != nil { return nil, err }
		return crypto.UnmarshalSecp256k1PrivateKey(keyBytes)
	}
	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil { return nil, err }
	raw, _ := priv.Raw()
	os.WriteFile(KeyFile, []byte(hex.EncodeToString(raw)), 0600)
	return priv, nil
}

// --- 암호화 공통 유틸 ---

func deriveKey(priv crypto.PrivKey) []byte {
	raw, _ := priv.Raw()
	hash := sha256.Sum256(raw)
	return hash[:]
}

func encryptData(data []byte, priv crypto.PrivKey) ([]byte, error) {
	key := deriveKey(priv)
	block, err := aes.NewCipher(key)
	if err != nil { return nil, err }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return nil, err }
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil { return nil, err }
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func decryptData(data []byte, priv crypto.PrivKey) ([]byte, error) {
	key := deriveKey(priv)
	block, err := aes.NewCipher(key)
	if err != nil { return nil, err }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return nil, err }
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize { return nil, fmt.Errorf("data too short") }
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// --- 블록 저장/로드 ---

func SaveIdentitySecurely(priv crypto.PrivKey, block *blockchain.IdentityBlock) error {
	jsonData, _ := json.Marshal(block)
	encData, err := encryptData(jsonData, priv)
	if err != nil { return err }
	return os.WriteFile(IdentityFile, encData, 0600)
}

func LoadIdentitySecurely(priv crypto.PrivKey) (*blockchain.IdentityBlock, error) {
	encData, err := os.ReadFile(IdentityFile)
	if err != nil { return nil, err }
	jsonData, err := decryptData(encData, priv)
	if err != nil { return nil, err }
	var block blockchain.IdentityBlock
	err = json.Unmarshal(jsonData, &block)
	return &block, err
}

// --- 히스토리 저장/로드 (Updated to use helper functions) ---

func SaveHistory(priv crypto.PrivKey) error {
	if len(ChatHistory) == 0 {
		return fmt.Errorf("nothing to save")
	}
	data, err := json.Marshal(ChatHistory)
	if err != nil { return err }

    encData, err := encryptData(data, priv)
    if err != nil { return err }
    
	return os.WriteFile(HistoryFile, encData, 0644)
}

func LoadHistory(priv crypto.PrivKey) (int, error) {
    encData, err := os.ReadFile(HistoryFile)
    if err != nil { return 0, err }
    
    jsonData, err := decryptData(encData, priv)
    if err != nil { return 0, err }

	if err := json.Unmarshal(jsonData, &ChatHistory); err != nil { return 0, err }
	return len(ChatHistory), nil
}
