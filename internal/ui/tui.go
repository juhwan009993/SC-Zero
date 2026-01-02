package ui

import (
	"context"
	"fmt"
	"sc-zero/internal/p2p"
	"sc-zero/internal/storage"
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
			if text == "" { return }
			ui.handleInput(text)
			ui.Input.SetText("")
		}
	})

	flex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(ui.ChatView, 0, 1, false).AddItem(ui.Input, 3, 1, true)
	ui.App.SetRoot(flex, true)

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
		switch args[0] {
		case "/exit":
			ui.App.Stop()
		case "/connect":
			if len(args) < 2 {
				ui.LogChan <- "[red]Usage: /connect <addr>[-]"
				return
			}
			go ui.Node.Connect(context.Background(), args[1])
		}
	} else {
		ui.MsgChan <- text
		ui.LogChan <- fmt.Sprintf("[yellow]Me: %s[-]", text)
		storage.AppendLog("Me", text)
	}
}
