package blockchain

import (
	"errors"
	"fmt"
	"sc-zero/internal/core"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p/core/crypto"
)

type Ledger struct {
	VerifiedPeers map[string]*IdentityBlock
	Mutex         sync.RWMutex
}

var GlobalLedger = &Ledger{
	VerifiedPeers: make(map[string]*IdentityBlock),
}

// VerifyAndAdd: 블록 검증 후 장부 등록
func (l *Ledger) VerifyAndAdd(block *IdentityBlock) error {
	l.Mutex.Lock()
	defer l.Mutex.Unlock()

	// 1. 이미 있으면 패스 (업데이트 로직은 생략)
	if _, exists := l.VerifiedPeers[block.PeerID]; exists {
		return nil
	}

	// 2. PoW 검증
	target := strings.Repeat("0", core.Difficulty)
	if !strings.HasPrefix(block.Hash, target) {
		return errors.New("PoW failed: Insufficient work")
	}

	// 3. 해시 무결성 검증
	if block.CalculateHash() != block.Hash {
		return errors.New("Hash mismatch: Data corrupted")
	}

	// 4. 서명 검증 (스푸핑 방지)
	pubKey, err := crypto.UnmarshalPublicKey(block.PubKey)
	if err != nil {
		return err
	}
	signData := []byte(fmt.Sprintf("%s%d", block.PeerID, block.Timestamp))
	valid, err := pubKey.Verify(signData, block.Signature)
	if err != nil || !valid {
		return errors.New("Signature invalid: Spoofing attempt")
	}

	// 5. 등록
	l.VerifiedPeers[block.PeerID] = block
	return nil
}

func (l *Ledger) IsVerified(peerID string) bool {
	l.Mutex.RLock()
	defer l.Mutex.RUnlock()
	_, exists := l.VerifiedPeers[peerID]
	return exists
}

func (l *Ledger) GetAllPeerIDs() []string {
	l.Mutex.RLock()
	defer l.Mutex.RUnlock()
	keys := make([]string, 0, len(l.VerifiedPeers))
	for k := range l.VerifiedPeers {
		keys = append(keys, k)
	}
	return keys
}

func (l *Ledger) GetBlock(peerID string) *IdentityBlock {
	l.Mutex.RLock()
	defer l.Mutex.RUnlock()
	return l.VerifiedPeers[peerID]
}