package replica

import (
	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/types"
)

type Safety interface {
	ProcessBlock(block *blockchain.Block) error
	ProcessVote(vote *blockchain.Vote)
	ProcessRemoteTmo(tmo *pacemaker.TMO)
	ProcessLocalTmo(view types.View)
	GetChainStatus() string

	// MakeProposal is used to generate block in normal phase.
	MakeProposal(view types.View, payload []*message.Transaction) *blockchain.Block

	// MakePProposal is used to generate block in phalanx phase.
	MakePProposal(view types.View) *blockchain.Block
}
