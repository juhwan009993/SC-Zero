package ui

import (
	"context"
	"fmt"
	"sc-zero/internal/p2p"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type TUIManager struct {
	App      *tview.Application
	ChatView *tview.TextView
	Input    *tview.InputField
	MsgChan  chan string
	LogChan  chan string
	Node     *p2p.P2PManager
}

func NewTUIManager(msgChan, logChan chan string) *TUIManager {
	return &TUIManager{
		App:      tview.NewApplication(),
		MsgChan:  msgChan,
		LogChan:  logChan,
	}
}

func (ui *TUIManager) Init(node *p2p.P2PManager) {
	ui.Node = node
	ui.ChatView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetChangedFunc(func() { ui.App.Draw() })
	ui.ChatView.SetBorder(true).SetTitle(" SC-Zero Secure Messenger ")

	ui.Input = tview.NewInputField().SetLabel("> ").SetFieldBackgroundColor(tcell.ColorBlack)
	ui.Input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := ui.Input.GetText()
			if text == "" {
				return
			}
			ui.handleInput(text)
			ui.Input.SetText("")
		}
	})

	flex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(ui.ChatView, 0, 1, false).AddItem(ui.Input, 3, 1, true)
	ui.App.SetRoot(flex, true)

	// [추가됨] 시작 시 안내 메시지 출력
	ui.LogChan <- "[yellow]Welcome to SC-Zero! Type /help for commands.[-]"

	go func() {
		for msg := range ui.LogChan {
			ui.App.QueueUpdateDraw(func() { fmt.Fprintln(ui.ChatView, msg) })
		}
	}()
}

func (ui *TUIManager) Run() error { return ui.App.Run() }

func (ui *TUIManager) handleInput(text string) {
	if strings.HasPrefix(text, "/") {
		args := strings.Fields(text)
		cmd := args[0]

		switch cmd {
		// [추가됨] 도움말 명령어
		case "/help", "/?":
			ui.LogChan <- " "
			ui.LogChan <- "[white::b]=== Available Commands ===[-]"
			ui.LogChan <- "[yellow]/connect <addr>[-] : Connect to a peer using Multiaddr"
			ui.LogChan <- "[yellow]/exit[-]           : Exit the program safely"
			ui.LogChan <- "[yellow]/help[-]           : Show this help message"
			ui.LogChan <- " "

		case "/exit":
			ui.LogChan <- "[red]Shutting down...[-]"
			ui.App.Stop()

		case "/connect":
			if len(args) < 2 {
				ui.LogChan <- "[red]Usage: /connect <addr>[-]"
				return
			}
			// 연결 시도 로그 출력
			ui.LogChan <- fmt.Sprintf("[yellow]System: Connecting to %s...[-]", args[1])
			
			// 비동기로 연결 시도 (결과는 LogChan으로 들어옴)
			go func() {
				err := ui.Node.Connect(context.Background(), args[1])
				if err != nil {
					// 실패 시 에러 로그 전송
					ui.LogChan <- fmt.Sprintf("[red]Connection Failed: %v[-]", err)
				}
			}()

		default:
			ui.LogChan <- fmt.Sprintf("[red]Unknown command: %s. Type /help[-]", cmd)
		}
	} else {
		// 일반 메시지 전송
		ui.MsgChan <- text
		ui.LogChan <- fmt.Sprintf("[yellow]Me: %s[-]", text)
	}
}
