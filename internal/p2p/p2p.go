package p2p

import (
	"bufio"
	"context"
	"fmt"
	"sc-zero/internal/core"
	"sc-zero/internal/storage"
	"strings"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type P2PManager struct {
	Host    host.Host
	MsgChan chan string // 보내는 메시지 (UI -> Network)
	LogChan chan string // 시스템/수신 로그 (Network -> UI)
}

func NewP2PManager(priv crypto.PrivKey, msgChan, logChan chan string) (*P2PManager, error) {
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.Identity(priv),
	)
	if err != nil {
		return nil, err
	}

	m := &P2PManager{
		Host:    h,
		MsgChan: msgChan,
		LogChan: logChan,
	}

	h.SetStreamHandler(core.ProtocolID, m.handleStream)
	return m, nil
}

func (m *P2PManager) Connect(targetAddr string) {
	m.LogChan <- fmt.Sprintf("[System] Dialing %s...", targetAddr)
	
	ma, err := multiaddr.NewMultiaddr(targetAddr)
	if err != nil {
		m.LogChan <- fmt.Sprintf("[red][Error] Invalid Address: %v[-]", err)
		return
	}
	info, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		m.LogChan <- fmt.Sprintf("[red][Error] Peer Info Error: %v[-]", err)
		return
	}

	if err := m.Host.Connect(context.Background(), *info); err != nil {
		m.LogChan <- fmt.Sprintf("[red][Error] Connection Failed: %v[-]", err)
		return
	}

	s, err := m.Host.NewStream(context.Background(), info.ID, core.ProtocolID)
	if err != nil {
		m.LogChan <- fmt.Sprintf("[red][Error] Stream Failed: %v[-]", err)
		return
	}

	m.LogChan <- "[green][System] 🚀 Ready to chat![-]"
	go m.readLoop(s)
	go m.writeLoop(s)
}

func (m *P2PManager) handleStream(s network.Stream) {
	m.LogChan <- "[green][System] 🔔 Secure Connection Established![-]"
	go m.readLoop(s)
	go m.writeLoop(s)
}

func (m *P2PManager) readLoop(s network.Stream) {
	reader := bufio.NewReader(s)
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			m.LogChan <- "[red][System] Disconnected.[-]"
			return
		}
		content := strings.TrimSpace(str)
		storage.AppendLog("Peer", content)
		m.LogChan <- fmt.Sprintf("[cyan]Peer: %s[-]", content)
	}
}

func (m *P2PManager) writeLoop(s network.Stream) {
	writer := bufio.NewWriter(s)
	for msg := range m.MsgChan {
		_, err := writer.WriteString(msg + "\n")
		if err != nil {
			m.LogChan <- "[red][System] Send Failed.[-]"
			return
		}
		writer.Flush()
	}
}