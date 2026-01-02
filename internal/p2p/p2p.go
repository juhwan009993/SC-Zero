package p2p

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sc-zero/internal/blockchain"
	"sc-zero/internal/core"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type P2PManager struct {
	Host      host.Host
	MsgChan   chan string
	LogChan   chan string
	MyIDBlock *blockchain.IdentityBlock
}

func NewP2PManager(priv crypto.PrivKey, msgChan, logChan chan string, myBlock *blockchain.IdentityBlock) (*P2PManager, error) {
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.Identity(priv),
	)
	if err != nil {
		return nil, err
	}

	m := &P2PManager{
		Host:      h,
		MsgChan:   msgChan,
		LogChan:   logChan,
		MyIDBlock: myBlock,
	}

	h.SetStreamHandler(core.ProtocolID, m.handleStream)
	return m, nil
}

// Connect: CLI에서 호출되며, 연결 실패 시 error를 반환합니다.
func (m *P2PManager) Connect(ctx context.Context, targetAddr string) error {
	// 1. 주소 파싱
	ma, err := multiaddr.NewMultiaddr(targetAddr)
	if err != nil {
		return fmt.Errorf("invalid address format: %w", err)
	}

	// 2. 피어 정보 추출
	info, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	// 3. 연결 시도 (Blocking)
	// 이 함수가 에러 없이 리턴되어야 main.go에서 TUI를 실행합니다.
	if err := m.Host.Connect(ctx, *info); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	// 4. 스트림 개방
	s, err := m.Host.NewStream(ctx, info.ID, core.ProtocolID)
	if err != nil {
		return fmt.Errorf("stream creation failed: %w", err)
	}

	// 5. 성공 로그 및 비동기 처리 시작
	m.LogChan <- fmt.Sprintf("[green]Connected to %s[-]", targetAddr)
	m.LogChan <- "[green]Performing Handshake...[-]"
	
	go m.readLoop(s)
	go m.writeLoop(s)

	return nil
}

func (m *P2PManager) handleStream(s network.Stream) {
	m.LogChan <- "[green]Inbound Connection! Performing Handshake...[-]"
	go m.readLoop(s)
	go m.writeLoop(s)
}

func (m *P2PManager) writeLoop(s network.Stream) {
	writer := bufio.NewWriter(s)

	// 1. [Handshake] 내 신원 블록 전송 (AUTH)
	m.sendPacket(writer, "AUTH", m.MyIDBlock.ToJSON())

	// 2. [Gossip] 내가 아는 피어 목록 전송
	knowns := blockchain.GlobalLedger.GetAllPeerIDs()
	knownsJSON, _ := json.Marshal(knowns)
	m.sendPacket(writer, "GOSSIP", string(knownsJSON))

	// 3. 메시지 전송 루프
	for msg := range m.MsgChan {
		m.sendPacket(writer, "MSG", msg)
	}
}

func (m *P2PManager) readLoop(s network.Stream) {
	reader := bufio.NewReader(s)
	remotePID := s.Conn().RemotePeer().String()

	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			m.LogChan <- "[red]Disconnected.[-]"
			return
		}

		var packet core.NetworkPacket
		if err := json.Unmarshal([]byte(str), &packet); err != nil {
			continue
		}

		switch packet.Type {
		case "AUTH":
			block, _ := blockchain.FromJSON(packet.Payload)
			if err := blockchain.GlobalLedger.VerifyAndAdd(block); err != nil {
				m.LogChan <- fmt.Sprintf("[red]Identity Verification Failed: %v[-]", err)
			} else {
				m.LogChan <- fmt.Sprintf("[green]Verified Identity: %s (PoW: %s...)[-]", block.PeerID[:8], block.Hash[:8])
			}

		case "GOSSIP":
			var remoteList []string
			json.Unmarshal([]byte(packet.Payload), &remoteList)
			// 내가 모르는 피어에 대해 블록 요청
			for _, pid := range remoteList {
				if !blockchain.GlobalLedger.IsVerified(pid) && pid != m.Host.ID().String() {
					m.sendPacket(bufio.NewWriter(s), "REQUEST", pid)
				}
			}

		case "REQUEST":
			// 상대방이 요청한 피어의 블록을 보내줌
			block := blockchain.GlobalLedger.GetBlock(packet.Payload)
			if block != nil {
				m.sendPacket(bufio.NewWriter(s), "RESPONSE", block.ToJSON())
			}

		case "RESPONSE":
			// 요청했던 블록 수신 및 검증
			block, _ := blockchain.FromJSON(packet.Payload)
			if err := blockchain.GlobalLedger.VerifyAndAdd(block); err == nil {
				m.LogChan <- fmt.Sprintf("[yellow]Synced Identity: %s[-]", block.PeerID[:8])
			}

		case "MSG":
			// 인증된 사용자인지 체크 후 메시지 출력
			prefix := "[Unverified]"
			if blockchain.GlobalLedger.IsVerified(remotePID) {
				prefix = "[Verified]"
			}
			m.LogChan <- fmt.Sprintf("%s[cyan]%s: %s[-]", prefix, remotePID[:6], packet.Payload)
		}
	}
}

func (m *P2PManager) sendPacket(w *bufio.Writer, typeStr, payload string) {
	packet := core.NetworkPacket{Type: typeStr, Payload: payload}
	bytes, _ := json.Marshal(packet)
	w.WriteString(string(bytes) + "\n")
	w.Flush()
}