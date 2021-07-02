package blockchain

import (
	"time"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/types"

	pCommonProto "github.com/Grivn/phalanx/common/protos"
)

type Block struct {
	types.View
	QC        *QC
	Proposer  identity.NodeID
	Timestamp time.Time
	Payload   []*message.Transaction
	PrevID    crypto.Identifier
	Sig       crypto.Signature
	ID        crypto.Identifier
	PBatch    *pCommonProto.PartialOrderBatch
}

type rawBlock struct {
	types.View
	QC       *QC
	Proposer identity.NodeID
	Payload  []string
	PrevID   crypto.Identifier
	Sig      crypto.Signature
	ID       crypto.Identifier
}

type rawPBlock struct {
	types.View
	QC       *QC
	Proposer identity.NodeID
	Payload  *pCommonProto.PartialOrderBatch
	PrevID   crypto.Identifier
	Sig      crypto.Signature
	ID       crypto.Identifier
}

// MakeBlock creates an unsigned block
func MakeBlock(view types.View, qc *QC, prevID crypto.Identifier, payload []*message.Transaction, proposer identity.NodeID) *Block {
	b := new(Block)
	b.View = view
	b.Proposer = proposer
	b.QC = qc
	b.Payload = payload
	b.PrevID = prevID
	b.makeID(proposer)
	return b
}

// MakePBlock creates an unsigned block with phalanx payload
func MakePBlock(view types.View, qc *QC, prevID crypto.Identifier, pBatch *pCommonProto.PartialOrderBatch, proposer identity.NodeID) *Block {
	b := new(Block)
	b.View = view
	b.Proposer = proposer
	b.QC = qc
	b.PBatch = pBatch
	b.PrevID = prevID
	b.makePID(proposer)
	return b
}

func (b *Block) makeID(nodeID identity.NodeID) {
	raw := &rawBlock{
		View:     b.View,
		QC:       b.QC,
		Proposer: b.Proposer,
		PrevID:   b.PrevID,
	}
	var payloadIDs []string
	for _, txn := range b.Payload {
		payloadIDs = append(payloadIDs, txn.ID)
	}
	raw.Payload = payloadIDs
	b.ID = crypto.MakeID(raw)
	// TODO: uncomment the following
	b.Sig, _ = crypto.PrivSign(crypto.IDToByte(b.ID), nodeID, nil)
}

func (b *Block) makePID(nodeID identity.NodeID) {
	raw := &rawPBlock{
		View:     b.View,
		QC:       b.QC,
		Proposer: b.Proposer,
		PrevID:   b.PrevID,
		Payload:  b.PBatch,
	}
	b.ID = crypto.MakeID(raw)
	// TODO: uncomment the following
	b.Sig, _ = crypto.PrivSign(crypto.IDToByte(b.ID), nodeID, nil)
}
