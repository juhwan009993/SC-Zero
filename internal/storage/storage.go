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
	"sc-zero/internal/core"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// 전역 변수로 메모리상 대화 기록 관리
var ChatHistory []core.ChatMessage

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

// 키 로드 또는 생성
func LoadOrGenerateKey(filename string) (crypto.PrivKey, error) {
	if _, err := os.Stat(filename); err == nil {
		data, err := os.ReadFile(filename)
		if err != nil { return nil, err }
		keyBytes, err := hex.DecodeString(string(data))
		if err != nil { return nil, err }
		return crypto.UnmarshalSecp256k1PrivateKey(keyBytes)
	}
	fmt.Println("[Storage] Generating new identity key...")
	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil { return nil, err }
	raw, _ := priv.Raw()
	keyHex := hex.EncodeToString(raw)
	os.WriteFile(filename, []byte(keyHex), 0600)
	return priv, nil
}

// 저장소 암호화 키 파생
func deriveStorageKey(priv crypto.PrivKey) []byte {
	raw, _ := priv.Raw()
	hash := sha256.Sum256(raw)
	return hash[:]
}

// 암호화 저장
func SaveHistory(priv crypto.PrivKey, filename string) error {
	if len(ChatHistory) == 0 {
		return fmt.Errorf("nothing to save")
	}
	data, err := json.Marshal(ChatHistory)
	if err != nil { return err }

	key := deriveStorageKey(priv)
	block, err := aes.NewCipher(key)
	if err != nil { return err }

	gcm, err := cipher.NewGCM(block)
	if err != nil { return err }

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return err }

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return os.WriteFile(filename, ciphertext, 0644)
}

// 복호화 로드
func LoadHistory(priv crypto.PrivKey, filename string) (int, error) {
	ciphertext, err := os.ReadFile(filename)
	if err != nil { return 0, err }

	key := deriveStorageKey(priv)
	block, err := aes.NewCipher(key)
	if err != nil { return 0, err }

	gcm, err := cipher.NewGCM(block)
	if err != nil { return 0, err }

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize { return 0, fmt.Errorf("ciphertext too short") }

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil { return 0, fmt.Errorf("decryption failed") }

	if err := json.Unmarshal(plaintext, &ChatHistory); err != nil { return 0, err }
	return len(ChatHistory), nil
}
