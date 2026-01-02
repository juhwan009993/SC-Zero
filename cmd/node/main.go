package main

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const (
	protocolID  = "/sc-zero/1.0.0"
	keyFile     = "identity.key"   // [변경됨] 신원 증명 키 파일
	historyFile = "chat_history.enc" // 암호화된 대화 저장 파일
)

// 대화 내용 구조체
type ChatMessage struct {
	Sender    string    `json:"sender"`    // 보낸 사람 (Peer ID)
	Content   string    `json:"content"`   // 내용
	Timestamp time.Time `json:"timestamp"` // 시간
}

// 전역 변수: 대화 기록 저장소
var chatHistory []ChatMessage

func main() {
	ctx := context.Background()

	// 1. 개인키 로드 (없으면 생성)
	privKey, err := loadOrGenerateKey(keyFile)
	if err != nil {
		panic(err)
	}

	// 2. SC-Zero 노드 구동
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.Identity(privKey),
	)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	h.SetStreamHandler(protocolID, handleStream)

	// 3. UI 출력
	fmt.Println("====================================================")
	fmt.Println("       SC-Zero : Encrypted Storage Ver")
	fmt.Println("====================================================")
	fmt.Printf("Node ID : %s\n", h.ID())
	fmt.Println("----------------------------------------------------")
	
	fmt.Println("My Addresses (Copy this to peer):")
	for _, addr := range h.Addrs() {
		// IP주소/포트/ID가 합쳐진 전체 주소 출력
		fmt.Printf(" - %s/p2p/%s\n", addr, h.ID())
	}
	fmt.Println("----------------------------------------------------")
	
	// 자동 로드 시도
	if _, err := os.Stat(historyFile); err == nil {
		fmt.Println("[System] Found encrypted history. Loading...")
		loadHistory(privKey)
	}

	fmt.Println("Available Commands:")
	fmt.Println(" - /save : Encrypt & Save chat history")
	fmt.Println(" - /load : Decrypt & Load chat history")
	fmt.Println(" - /history : Show past logs")
	fmt.Println("====================================================")

	// 4. 모드 선택
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\n[Mode Selection]")
		fmt.Println("1. Wait (Listen)")
		fmt.Println("2. Connect (Dial)")
		fmt.Print("Select> ")
		
		if !scanner.Scan() { return }
		choice := strings.TrimSpace(scanner.Text())

		if choice == "1" {
			fmt.Println("[Info] Listening for incoming connections...")
			select {} 
		} else if choice == "2" {
			fmt.Print("Target Address> ")
			if !scanner.Scan() { return }
			targetAddr := strings.TrimSpace(scanner.Text())
			if targetAddr == "" { continue }
			
			connectToPeer(ctx, h, targetAddr)
			break 
		}
	}
}

// ---------------------------------------------------------
// [Storage Logic] 암호화 저장 및 로드
// ---------------------------------------------------------

// 대화 내용을 메모리에 추가
func appendLog(sender string, content string) {
	msg := ChatMessage{
		Sender:    sender,
		Content:   content,
		Timestamp: time.Now(),
	}
	chatHistory = append(chatHistory, msg)
}

// 개인키를 이용해 저장소 암호화 키 파생 (SHA-256)
func deriveStorageKey(priv crypto.PrivKey) []byte {
	raw, _ := priv.Raw()
	hash := sha256.Sum256(raw)
	return hash[:] // 32 bytes for AES-256
}

// 히스토리 저장 (AES-GCM 암호화)
func saveHistory(priv crypto.PrivKey) {
	if len(chatHistory) == 0 {
		fmt.Println("[System] Nothing to save.")
		return
	}

	// 1. JSON 직렬화
	data, err := json.Marshal(chatHistory)
	if err != nil {
		fmt.Println("[Error] JSON marshal failed:", err)
		return
	}

	// 2. 키 파생
	key := deriveStorageKey(priv)

	// 3. AES-GCM 암호화
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println("[Error] Cipher creation failed:", err)
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		fmt.Println("[Error] GCM creation failed:", err)
		return
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		fmt.Println("[Error] Nonce generation failed:", err)
		return
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// 4. 파일 쓰기
	if err := os.WriteFile(historyFile, ciphertext, 0644); err != nil {
		fmt.Println("[Error] File write failed:", err)
		return
	}
	fmt.Printf("[System] Chat history encrypted and saved to '%s'\n", historyFile)
}

// 히스토리 로드 (복호화)
func loadHistory(priv crypto.PrivKey) {
	// 1. 파일 읽기
	ciphertext, err := os.ReadFile(historyFile)
	if err != nil {
		fmt.Println("[Error] Read file failed:", err)
		return
	}

	// 2. 키 파생
	key := deriveStorageKey(priv)

	// 3. AES-GCM 복호화
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println("[Error] Cipher failed:", err)
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		fmt.Println("[Error] GCM failed:", err)
		return
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		fmt.Println("[Error] Ciphertext too short")
		return
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		fmt.Println("[Error] Decryption failed! Key mismatch or corrupted file.")
		return
	}

	// 4. JSON 파싱
	if err := json.Unmarshal(plaintext, &chatHistory); err != nil {
		fmt.Println("[Error] JSON unmarshal failed:", err)
		return
	}
	fmt.Printf("[System] Loaded %d past messages.\n", len(chatHistory))
}

// 과거 기록 보기
func printHistory() {
	fmt.Println("\n--- Chat History ---")
	for _, msg := range chatHistory {
		ts := msg.Timestamp.Format("15:04:05")
		sender := "Peer"
		if msg.Sender == "Me" {
			sender = "Me"
		}
		fmt.Printf("[%s] %s: %s\n", ts, sender, msg.Content)
	}
	fmt.Println("--------------------")
}

// ---------------------------------------------------------
// [Helper] 키 로드
// ---------------------------------------------------------
func loadOrGenerateKey(filename string) (crypto.PrivKey, error) {
	if _, err := os.Stat(filename); err == nil {
		data, err := os.ReadFile(filename)
		if err != nil { return nil, err }
		keyBytes, err := hex.DecodeString(string(data))
		if err != nil { return nil, err }
		return crypto.UnmarshalSecp256k1PrivateKey(keyBytes)
	}
	// 생성
	fmt.Println("[System] Generating new identity key...")
	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil { return nil, err }
	raw, _ := priv.Raw()
	keyHex := hex.EncodeToString(raw)
	os.WriteFile(filename, []byte(keyHex), 0600)
	return priv, nil
}

// ---------------------------------------------------------
// [Communication Logic] 채팅 처리
// ---------------------------------------------------------

func handleStream(s network.Stream) {
	fmt.Println("\n[System] Connection Established!")
	go readData(s)
	writeData(s)
}

func connectToPeer(ctx context.Context, h host.Host, targetAddr string) {
	maddr, err := multiaddr.NewMultiaddr(targetAddr)
	if err != nil { return }
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil { return }

	if err := h.Connect(ctx, *info); err != nil { return }
	s, err := h.NewStream(ctx, info.ID, protocolID)
	if err != nil { return }

	fmt.Println("[System] Ready to chat!")
	go readData(s)
	writeData(s)
}

func readData(s network.Stream) {
	reader := bufio.NewReader(s)
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("\n[System] Disconnected.")
			os.Exit(0)
		}
		content := strings.TrimSpace(str)
		appendLog("Peer", content) 
		fmt.Printf("\rPeer: %s\n> ", content)
	}
}

func writeData(s network.Stream) {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(s)
	
	// 키 로드 (파일명 변경 반영)
	privKey, _ := loadOrGenerateKey(keyFile)

	fmt.Print("> ")
	for scanner.Scan() {
		text := scanner.Text()
		
		if text == "/save" {
			saveHistory(privKey)
			fmt.Print("> ")
			continue
		} else if text == "/load" {
			loadHistory(privKey)
			printHistory()
			fmt.Print("> ")
			continue
		} else if text == "/history" {
			printHistory()
			fmt.Print("> ")
			continue
		}

		appendLog("Me", text)
		_, err := writer.WriteString(text + "\n")
		if err != nil { break }
		writer.Flush()
		fmt.Print("> ")
	}
}