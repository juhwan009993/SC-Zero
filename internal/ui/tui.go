package ui

import (
	"fmt"
	"os" // 파일 존재 여부 확인용
	"sc-zero/internal/core"
	"sc-zero/internal/storage"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/rivo/tview"
)

type TUIManager struct {
	App       *tview.Application
	ChatView  *tview.TextView
	InputView *tview.InputField
	MsgChan   chan string // User Input -> Network
	LogChan   chan string // Network/System -> UI
	PrivKey   crypto.PrivKey
}

func NewTUIManager(priv crypto.PrivKey, msgChan, logChan chan string) *TUIManager {
	return &TUIManager{
		App:     tview.NewApplication(),
		MsgChan: msgChan,
		LogChan: logChan,
		PrivKey: priv,
	}
}

func (ui *TUIManager) Init() {
	// 1. 채팅 뷰
	ui.ChatView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	ui.ChatView.SetBorder(true).SetTitle(" Chat History ")

	// 2. 입력 뷰
	ui.InputView = tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack)
	ui.InputView.SetBorder(true).SetTitle(" Input ")

	ui.InputView.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter { return }
		text := strings.TrimSpace(ui.InputView.GetText())
		if text == "" { return }

		if strings.HasPrefix(text, "/") {
			// 명령어 처리 (고루틴)
			go func() { ui.handleCommand(text) }()
		} else {
			// 메시지 전송
			select {
			case ui.MsgChan <- text:
				storage.AppendLog("Me", text)
				go func() { ui.PrintLog(fmt.Sprintf("[yellow]Me: %s[-]", text)) }()
			default:
				go func() { ui.PrintLog("[red][System] Channel full or no connection.[-]") }()
			}
		}
		ui.InputView.SetText("")
	})

	// 3. 레이아웃
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.ChatView, 0, 1, false).
		AddItem(ui.InputView, 3, 1, true)

	ui.App.SetRoot(flex, true).EnableMouse(true)

	// 로그 수신 루프
	go func() {
		for msg := range ui.LogChan {
			ui.PrintLog(msg)
		}
	}()

	// [자동 로드] 파일이 존재하면 시작 시 자동으로 불러오기
	if _, err := os.Stat(core.HistoryFile); err == nil {
		count, err := storage.LoadHistory(ui.PrivKey, core.HistoryFile)
		if err == nil {
			ui.PrintLog(fmt.Sprintf("[System] Auto-loaded %d messages.", count))
			ui.printHistory()
		} else {
			ui.PrintLog(fmt.Sprintf("[red][Error] Auto-load failed: %v[-]", err))
		}
	}
}

func (ui *TUIManager) Run() error {
	return ui.App.Run()
}

func (ui *TUIManager) PrintLog(msg string) {
	ui.App.QueueUpdateDraw(func() {
		fmt.Fprintln(ui.ChatView, msg)
		ui.ChatView.ScrollToEnd()
	})
}

func (ui *TUIManager) handleCommand(cmd string) {
	switch cmd {
	// [변경] /save, /load는 제거됨.
	
	case "/close":
		// [자동 저장 및 종료]
		err := storage.SaveHistory(ui.PrivKey, core.HistoryFile)
		if err != nil {
			// TUI 종료 후 터미널에 에러를 남기기 위해 Println 사용
			fmt.Printf("\n[Error] Save failed: %v\n", err)
		} else {
			fmt.Printf("\n[System] Chat history saved to '%s'.\n", core.HistoryFile)
		}
		ui.App.Stop() // 앱 종료

	case "/history":
		ui.printHistory()

	default:
		ui.PrintLog("[red][System] Unknown command. Use /close to save & exit.[-]")
	}
}

func (ui *TUIManager) printHistory() {
	ui.PrintLog("--- 📜 Past History ---")
	for _, msg := range storage.GetHistory() {
		ts := msg.Timestamp.Format("15:04")
		color := "cyan"
		if msg.Sender == "Me" { color = "yellow" }
		ui.PrintLog(fmt.Sprintf("[%s][%s]%s: %s[-]", ts, color, msg.Sender, msg.Content))
	}
	ui.PrintLog("-----------------------")
}