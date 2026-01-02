package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"sc-zero/internal/blockchain"
	"sc-zero/internal/p2p"
	"sc-zero/internal/storage"
	"sc-zero/internal/ui"

	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	// --- 1. 초기화 및 키 로드 ---
	fmt.Println("========================================")
	fmt.Println("   SC-Zero : Secure Identity Messenger  ")
	fmt.Println("========================================")

	privKey, err := storage.LoadOrGenerateKey()
	if err != nil { panic(err) }

	var myBlock *blockchain.IdentityBlock
	myBlock, err = storage.LoadIdentitySecurely(privKey)
	if err != nil {
		fmt.Println("[Init] No existing identity found.")
		fmt.Println("[Init] Mining new Identity Block (PoW)... This may take a few seconds.")
		pid, _ := peer.IDFromPrivateKey(privKey)
		myBlock, err = blockchain.CreateIdentityBlock(privKey, pid.String())
		if err != nil { panic(err) }
		storage.SaveIdentitySecurely(privKey, myBlock)
		fmt.Printf("[Init] Identity Mined! Hash: %s\n", myBlock.Hash)
	} else {
		fmt.Printf("[Init] Identity Loaded! Hash: %s\n", myBlock.Hash)
	}

	// --- 2. P2P 매니저 생성 ---
	msgChan := make(chan string, 100)
	logChan := make(chan string, 100)

	node, err := p2p.NewP2PManager(privKey, msgChan, logChan, myBlock)
	if err != nil { panic(err) }
	defer node.Host.Close()

	// --- 3. 주소 출력 ---
	fmt.Println("\n[ My Node Addresses ]")
	for _, addr := range node.Host.Addrs() {
		fmt.Printf("- %s/p2p/%s\n", addr, node.Host.ID())
	}
	fmt.Println("----------------------------------------")

	// --- 4. [핵심 변경] 모드 선택 (CLI) ---
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Println("\nSelect Mode:")
		fmt.Println("1. Wait for connection (Listen)")
		fmt.Println("2. Connect to peer")
		fmt.Print("Choice> ")
		
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		if choice == "1" {
			// 대기 모드: 바로 TUI 진입
			fmt.Println("Entering Wait Mode...")
			time.Sleep(1 * time.Second)
			break 
		} else if choice == "2" {
			// 연결 모드: 주소 입력 받기
			fmt.Print("Enter Target Address: ")
			target, _ := reader.ReadString('\n')
			target = strings.TrimSpace(target)

			if target == "" {
				fmt.Println("[!] Address cannot be empty.")
				continue
			}

			fmt.Printf("Connecting to %s...\n", target)
			
			// 연결 시도 (여기서 멈춤)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := node.Connect(ctx, target)
			cancel()

			if err != nil {
				fmt.Printf("[!] Connection Failed: %v\n", err)
				fmt.Println("Please try again.")
				continue // 다시 메뉴로 돌아감
			} else {
				fmt.Println("[+] Connected successfully!")
				time.Sleep(1 * time.Second)
				break // 루프 탈출 -> TUI 진입
			}
		} else {
			fmt.Println("[!] Invalid choice.")
		}
	}

	// --- 5. TUI 실행 ---
	tui := ui.NewTUIManager(msgChan, logChan)
	tui.Init(node)

	if err := tui.Run(); err != nil {
		panic(err)
	}
}