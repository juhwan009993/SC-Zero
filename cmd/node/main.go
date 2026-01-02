package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/multiformats/go-multiaddr" // [추가됨] 주소 타입 사용을 위해 필요

	"sc-zero/internal/core"
	"sc-zero/internal/p2p"
	"sc-zero/internal/storage"
	"sc-zero/internal/ui"
)

func main() {
	// 1. 채널 생성 (모듈 간 통신용)
	msgChan := make(chan string, 100) // UI -> P2P
	logChan := make(chan string, 100) // P2P/System -> UI

	// 2. 키 로드
	privKey, err := storage.LoadOrGenerateKey(core.KeyFile)
	if err != nil {
		panic(err)
	}

	// 3. P2P 매니저 초기화
	node, err := p2p.NewP2PManager(privKey, msgChan, logChan)
	if err != nil {
		panic(err)
	}
	defer node.Host.Close()

	// 4. CLI 모드 (초기 정보 출력 및 모드 선택)
	printBanner(node.Host.ID().String(), node.Host.Addrs())
	
	startMode, targetAddr := selectMode()

	// 5. UI 매니저 초기화 및 실행
	tui := ui.NewTUIManager(privKey, msgChan, logChan)
	tui.Init()

	// 초기 상태 로그 전송
	logChan <- "[System] SC-Zero TUI Started."
	logChan <- fmt.Sprintf("[System] Status: %s", startMode)

	// 연결 시도 (비동기)
	if targetAddr != "" {
		go func() {
			time.Sleep(1 * time.Second)
			node.Connect(targetAddr)
		}()
	}

	// 앱 실행 (Blocking)
	if err := tui.Run(); err != nil {
		panic(err)
	}
}

// CLI 헬퍼 함수들
// [수정됨] addrs의 타입을 []interface{}에서 []multiaddr.Multiaddr로 변경
func printBanner(id string, addrs []multiaddr.Multiaddr) {
	fmt.Println("====================================================")
	fmt.Println("       SC-Zero : Modular TUI Edition")
	fmt.Println("====================================================")
	fmt.Printf("Node ID : %s\n", id)
	fmt.Println("My Addresses:")
	for _, addr := range addrs {
		fmt.Printf(" - %s/p2p/%s\n", addr, id)
	}
	fmt.Println("----------------------------------------------------")
}

func selectMode() (string, string) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\n[Mode Selection]")
		fmt.Println("1. Wait (Listen)")
		fmt.Println("2. Connect (Dial)")
		fmt.Print("Select> ")
		
		if !scanner.Scan() { return "", "" }
		choice := strings.TrimSpace(scanner.Text())

		if choice == "1" {
			return "Listening...", ""
		} else if choice == "2" {
			fmt.Print("Target Address> ")
			if !scanner.Scan() { return "", "" }
			addr := strings.TrimSpace(scanner.Text())
			if addr != "" {
				return "Connecting...", addr
			}
		}
	}
}