package replica

import (
	"encoding/gob"
	"fmt"
	pCommonProto "github.com/Grivn/phalanx/common/protos"
	pCommonTypes "github.com/Grivn/phalanx/common/types"
	fhs "github.com/gitferry/bamboo/fasthostuff"
	"github.com/gitferry/bamboo/lbft"
	"time"

	"go.uber.org/atomic"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/election"
	"github.com/gitferry/bamboo/hotstuff"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/mempool"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/streamlet"
	"github.com/gitferry/bamboo/tchs"
	"github.com/gitferry/bamboo/types"
)

type Replica struct {
	node.Node
	Safety
	election.Election

	openPhalanx bool

	pd              *mempool.Producer
	pm              *pacemaker.Pacemaker
	start           chan bool // signal to start the node
	isStarted       atomic.Bool
	isByz           bool
	timer           *time.Timer // timeout for each view
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	eventChan       chan interface{}

	/* for monitoring node statistics */
	thrus                string
	lastViewTime         time.Time
	startTime            time.Time
	tmpTime              time.Time
	voteStart            time.Time
	totalCreateDuration  time.Duration
	totalProcessDuration time.Duration
	totalProposeDuration time.Duration
	totalDelay           time.Duration
	totalRoundTime       time.Duration
	totalVoteTime        time.Duration
	totalBlockSize       int
	receivedNo           int
	roundNo              int
	voteNo               int
	totalCommittedTx     int
	latencyNo            int
	proposedNo           int
	processedNo          int
	committedNo          int

	preTotalSafe int
	preTotalRisk int

	preTotalSafeM int
	preTotalRiskM int
}

// NewReplica creates a new replica instance
func NewReplica(id identity.NodeID, alg string, isByz bool) *Replica {
	r := new(Replica)
	r.Node = node.NewNode(id, isByz)
	if isByz {
		log.Infof("[%v] is Byzantine", r.ID())
	}
	if config.GetConfig().Master == "0" {
		r.Election = election.NewRotation(config.GetConfig().N())
	} else {
		r.Election = election.NewStatic(config.GetConfig().Master)
	}
	r.isByz = isByz
	r.pd = mempool.NewProducer()
	r.pm = pacemaker.NewPacemaker(config.GetConfig().N())
	r.start = make(chan bool)
	r.eventChan = make(chan interface{})
	r.committedBlocks = make(chan *blockchain.Block, 100)
	r.forkedBlocks = make(chan *blockchain.Block, 100)
	r.Register(blockchain.Block{}, r.HandleBlock)
	r.Register(blockchain.Vote{}, r.HandleVote)
	r.Register(pacemaker.TMO{}, r.HandleTmo)
	r.Register(message.Transaction{}, r.handleTxn)
	r.Register(message.Query{}, r.handleQuery)
	r.Register(pCommonProto.ConsensusMessage{}, r.HandleConsensusMessage)
	r.Register(pCommonProto.Command{}, r.HandleCommand)
	gob.Register(blockchain.Block{})
	gob.Register(blockchain.Vote{})
	gob.Register(pacemaker.TC{})
	gob.Register(pacemaker.TMO{})
	gob.Register(pCommonProto.ConsensusMessage{})
	gob.Register(pCommonProto.Command{})

	r.openPhalanx = true

	// Is there a better way to reduce the number of parameters?
	switch alg {
	case "hotstuff":
		r.Safety = hotstuff.NewHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	case "tchs":
		r.Safety = tchs.NewTchs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	case "streamlet":
		r.Safety = streamlet.NewStreamlet(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	case "lbft":
		r.Safety = lbft.NewLbft(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	case "fasthotstuff":
		r.Safety = fhs.NewFhs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	default:
		r.Safety = hotstuff.NewHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	}
	return r
}

/* Message Handlers */

func (r *Replica) HandleBlock(block blockchain.Block) {
	r.receivedNo++
	r.startSignal()
	log.Debugf("[%v] received a block from %v, view is %v, id: %x, prevID: %x", r.ID(), block.Proposer, block.View, block.ID, block.PrevID)
	r.eventChan <- block
}

func (r *Replica) HandleVote(vote blockchain.Vote) {
	if vote.View < r.pm.GetCurView() {
		return
	}
	r.startSignal()
	log.Debugf("[%v] received a vote frm %v, blockID is %x", r.ID(), vote.Voter, vote.BlockID)
	r.eventChan <- vote
}

func (r *Replica) HandleTmo(tmo pacemaker.TMO) {
	if tmo.View < r.pm.GetCurView() {
		return
	}
	log.Debugf("[%v] received a timeout from %v for view %v", r.ID(), tmo.NodeID, tmo.View)
	r.eventChan <- tmo
}

func (r *Replica) HandleConsensusMessage(message pCommonProto.ConsensusMessage) {
	r.startSignal()
	log.Debugf("[%v] received a consensus-message from %v", r.ID(), message.From)
	r.eventChan <- message
}

func (r *Replica) HandleCommand(command pCommonProto.Command) {
	r.startSignal()
	log.Debugf("[%v] received a command from %v", r.ID(), command.Author)
	r.eventChan <- command
}

// handleQuery replies a query with the statistics of the node
func (r *Replica) handleQuery(m message.Query) {
	aveCreateDuration := float64(r.totalCreateDuration.Milliseconds()) / float64(r.proposedNo)
	aveProcessTime := float64(r.totalProcessDuration.Milliseconds()) / float64(r.processedNo)
	aveVoteProcessTime := float64(r.totalVoteTime.Milliseconds()) / float64(r.voteNo)
	requestRate := float64(r.pd.TotalReceivedTxNo()) / time.Now().Sub(r.startTime).Seconds()
	aveRoundTime := float64(r.totalRoundTime.Milliseconds()) / float64(r.roundNo)

	// query essential information from node instance.
	nodeQuery := r.Node.QueryNode()

	phalanxMetrics := r.Node.QueryMetrics()

	committedCommandCount := phalanxMetrics.SafeCommandCount + phalanxMetrics.RiskCommandCount
	safeRate := float64(phalanxMetrics.SafeCommandCount) / float64(committedCommandCount) * 100
	riskRate := float64(phalanxMetrics.RiskCommandCount) / float64(committedCommandCount) * 100

	periodTotalSafe := phalanxMetrics.SafeCommandCount - r.preTotalSafe
	periodTotalRisk := phalanxMetrics.RiskCommandCount - r.preTotalRisk
	periodTotalCount := periodTotalRisk + periodTotalSafe
	periodSafeRate := float64(periodTotalSafe) / float64(periodTotalCount) * 100
	periodRiskRate := float64(periodTotalRisk) / float64(periodTotalCount) * 100

	r.preTotalSafe = phalanxMetrics.SafeCommandCount
	r.preTotalRisk = phalanxMetrics.RiskCommandCount

	committedCommandCountM := phalanxMetrics.MSafeCommandCount + phalanxMetrics.MRiskCommandCount
	safeRateM := float64(phalanxMetrics.MSafeCommandCount) / float64(committedCommandCountM) * 100
	riskRateM := float64(phalanxMetrics.MRiskCommandCount) / float64(committedCommandCountM) * 100

	periodTotalSafeM := phalanxMetrics.MSafeCommandCount - r.preTotalSafeM
	periodTotalRiskM := phalanxMetrics.MRiskCommandCount - r.preTotalRiskM
	periodTotalCountM := periodTotalRiskM + periodTotalSafeM
	periodSafeRateM := float64(periodTotalSafeM) / float64(periodTotalCountM) * 100
	periodRiskRateM := float64(periodTotalRiskM) / float64(periodTotalCountM) * 100

	r.preTotalSafeM = phalanxMetrics.MSafeCommandCount
	r.preTotalRiskM = phalanxMetrics.MRiskCommandCount

	r.thrus += fmt.Sprintf(
		"Time: %.2f s. "+
			"Throughput: %.2f txs/s, Latency: %.2f, "+
			"Safe Rate: %.2f%%, Risk Rate: %.2f%%, "+
			"Safe Rate (M): %.2f%%, Risk Rate (M): %.2f%%, "+
			"%.1fms(p), %.1fms(c), %.1fms(s), %.1fms(o)\n",
		time.Now().Sub(r.startTime).Seconds(),
		nodeQuery.Throughput, nodeQuery.Latency,
		periodSafeRate, periodRiskRate,
		periodSafeRateM, periodRiskRateM,
		phalanxMetrics.CurOrderLatency,
		phalanxMetrics.CurLogLatency,
		phalanxMetrics.CurCommitStreamLatency,
		phalanxMetrics.CurCommandInfoLatency,
	)

	totalCommands := phalanxMetrics.SafeCommandCount + phalanxMetrics.RiskCommandCount

	safeFrontAttackedRate := float64(phalanxMetrics.FrontAttackFromSafe) / float64(totalCommands) * 100
	riskFrontAttackedRate := float64(phalanxMetrics.FrontAttackFromRisk) / float64(totalCommands) * 100

	totalFrontAttackedCommands := phalanxMetrics.FrontAttackFromSafe + phalanxMetrics.FrontAttackFromRisk
	totalFrontAttackedRate := float64(totalFrontAttackedCommands) / float64(totalCommands) * 100

	totalFrontAttackedInterval := phalanxMetrics.FrontAttackIntervalSafe + phalanxMetrics.FrontAttackIntervalRisk
	totalFrontAttackedGiven := totalFrontAttackedCommands - totalFrontAttackedInterval
	totalFrontAttackedIntervalRate := float64(totalFrontAttackedInterval) / float64(totalFrontAttackedCommands) * 100
	totalFrontAttackedGivenRate := float64(totalFrontAttackedGiven) / float64(totalFrontAttackedCommands) * 100

	totalCommandsM := phalanxMetrics.MSafeCommandCount + phalanxMetrics.MRiskCommandCount

	safeFrontAttackedRateM := float64(phalanxMetrics.MFrontAttackFromSafe) / float64(totalCommandsM) * 100
	riskFrontAttackedRateM := float64(phalanxMetrics.MFrontAttackFromRisk) / float64(totalCommandsM) * 100

	totalFrontAttackedCommandsM := phalanxMetrics.MFrontAttackFromSafe + phalanxMetrics.MFrontAttackFromRisk
	totalFrontAttackedRateM := float64(totalFrontAttackedCommandsM) / float64(totalCommandsM) * 100

	totalFrontAttackedIntervalM := phalanxMetrics.MFrontAttackIntervalSafe + phalanxMetrics.MFrontAttackIntervalRisk
	totalFrontAttackedGivenM := totalFrontAttackedCommandsM - totalFrontAttackedIntervalM
	totalFrontAttackedIntervalRateM := float64(totalFrontAttackedIntervalM) / float64(totalFrontAttackedCommandsM) * 100
	totalFrontAttackedGivenRateM := float64(totalFrontAttackedGivenM) / float64(totalFrontAttackedCommandsM) * 100

	status := fmt.Sprintf(
		"chain status is: %s\n"+
			"Ave. block size is %v.\n"+
			"Ave. payload size is %v.\n"+
			"Ave. real block is %v.\n"+
			"Ave. creation time is %f ms.\n"+
			"Ave. processing time is %v ms.\n"+
			"Ave. vote time is %v ms.\n"+
			"Request rate is %f txs/s.\n"+
			"Ave. round time is %f ms.\n"+
			"Ave. Throughput is %f tx/s.\n"+
			"Ave. Latency is %f ms.\n"+
			"Ave. Latency of Phalanx\n"+
			"     Select Command %f ms.\n"+
			"     Generate Order Log %f ms.\n"+
			"     Commit Order Log %f ms.\n"+
			"     Commit Query Stream %f ms.\n"+
			"     Commit Command Info %f ms.\n"+
			"Ave. Order Size %d\n"+
			"Phalanx Command Rate\n"+
			"     Receive Rate %.2f\n"+
			"     Log Rate %.2f\n"+
			"     Gen Log Rate %.2f\n"+
			"     Total Commands %d\n"+
			"     Safe Committed Commands %d(%f%%)\n"+
			"     Risk Committed Commands %d(%f%%)\n"+
			"     Front Attacked Commands %d(%f%%)\n"+
			"     Front Attacked From Safe %d(%f%%)\n"+
			"     Front Attacked From Risk %d(%f%%)\n"+
			"     Front Attacked Given %d(%f%%)\n"+
			"     Front Attacked Interval %d(%f%%)\n"+
			"Phalanx Command Rate Medium\n"+
			"     Total Commands %d\n"+
			"     Safe Committed Commands %d(%f%%)\n"+
			"     Risk Committed Commands %d(%f%%)\n"+
			"     Front Attacked Commands %d(%f%%)\n"+
			"     Front Attacked From Safe %d(%f%%)\n"+
			"     Front Attacked From Risk %d(%f%%)\n"+
			"     Front Attacked Given %d(%f%%)\n"+
			"     Front Attacked Interval %d(%f%%)\n"+
			"Throughput is: \n%v",
		r.Safety.GetChainStatus(),
		nodeQuery.AveBlockSize,
		nodeQuery.AvePayloadSize,
		nodeQuery.AveRealBlock,
		aveCreateDuration,
		aveProcessTime,
		aveVoteProcessTime,
		requestRate,
		aveRoundTime,
		nodeQuery.TThroughput,
		nodeQuery.TLatency,
		phalanxMetrics.AvePackOrderLatency,
		phalanxMetrics.AveOrderLatency,
		phalanxMetrics.AveLogLatency,
		phalanxMetrics.AveCommitStreamLatency,
		phalanxMetrics.AveCommandInfoLatency,
		phalanxMetrics.AveOrderSize,
		phalanxMetrics.CommandPS,
		phalanxMetrics.LogPS,
		phalanxMetrics.GenLogPS,
		totalCommands,
		phalanxMetrics.SafeCommandCount, safeRate,
		phalanxMetrics.RiskCommandCount, riskRate,
		totalFrontAttackedCommands, totalFrontAttackedRate,
		phalanxMetrics.FrontAttackFromSafe, safeFrontAttackedRate,
		phalanxMetrics.FrontAttackFromRisk, riskFrontAttackedRate,
		totalFrontAttackedGiven, totalFrontAttackedGivenRate,
		totalFrontAttackedInterval, totalFrontAttackedIntervalRate,
		totalCommandsM,
		phalanxMetrics.MSafeCommandCount, safeRateM,
		phalanxMetrics.MRiskCommandCount, riskRateM,
		totalFrontAttackedCommandsM, totalFrontAttackedRateM,
		phalanxMetrics.MFrontAttackFromSafe, safeFrontAttackedRateM,
		phalanxMetrics.MFrontAttackFromRisk, riskFrontAttackedRateM,
		totalFrontAttackedGivenM, totalFrontAttackedGivenRateM,
		totalFrontAttackedIntervalM, totalFrontAttackedIntervalRateM,
		r.thrus,
	)
	m.Reply(message.QueryReply{Info: status})
}

func (r *Replica) handleTxn(m message.Transaction) {
	//payload, _ := json.Marshal(m)
	tx := pCommonTypes.GenerateTransaction(m.Command.Value)
	i := 0
	for {
		if i == config.GetConfig().DupRecv {
			break
		}
		r.Node.ReceiveTransaction(tx)
		r.pd.CalculateRequestTx()
		i++
	}
	r.startSignal()
	// the first leader kicks off the protocol
	if r.pm.GetCurView() == 0 && r.IsLeader(r.ID(), 1) {
		log.Debugf("[%v] is going to kick off the protocol", r.ID())
		r.pm.AdvanceView(0)
	}
}

/* Processors */

func (r *Replica) processCommittedBlock(block *blockchain.Block) {
	r.committedNo++
	r.totalCommittedTx += len(block.Payload)
	r.Node.CommitBlock()
	log.Infof("[%v] the block is committed, No. of transactions: %v, view: %v, current view: %v, id: %x", r.ID(), len(block.Payload), block.View, r.pm.GetCurView(), block.ID)
	if block.PBatch == nil {
		return
	}
	err := r.Node.CommitProposal(block.PBatch)
	if err != nil {
		panic(err)
	}
}

func (r *Replica) processForkedBlock(block *blockchain.Block) {
	if block.Proposer == r.ID() {
		for _, txn := range block.Payload {
			// collect txn back to mem pool
			r.pd.CollectTxn(txn)
		}
	}
	log.Infof("[%v] the block is forked, No. of transactions: %v, view: %v, current view: %v, id: %x", r.ID(), len(block.Payload), block.View, r.pm.GetCurView(), block.ID)
}

func (r *Replica) processNewView(newView types.View) {
	log.Debugf("[%v] is processing new view: %v, leader is %v", r.ID(), newView, r.FindLeaderFor(newView))
	if !r.IsLeader(r.ID(), newView) {
		return
	}
	r.proposeBlock(newView)
}

func (r *Replica) proposeBlock(view types.View) {
	createStart := time.Now()

	// generate different block types according to trusted target
	var block *blockchain.Block
	if r.openPhalanx && view > 1 {
		block = r.Safety.MakePProposal(view)
	} else {
		block = r.Safety.MakeProposal(view, r.pd.GeneratePayload())
	}

	r.totalBlockSize += len(block.Payload)
	r.proposedNo++
	createEnd := time.Now()
	createDuration := createEnd.Sub(createStart)
	block.Timestamp = time.Now()
	r.totalCreateDuration += createDuration
	r.Node.Broadcast(block)
	_ = r.Safety.ProcessBlock(block)
	r.voteStart = time.Now()
}

// ListenLocalEvent listens new view and timeout events
func (r *Replica) ListenLocalEvent() {
	r.lastViewTime = time.Now()
	r.timer = time.NewTimer(r.pm.GetTimerForView())
	for {
		r.timer.Reset(r.pm.GetTimerForView())
	L:
		for {
			select {
			case view := <-r.pm.EnteringViewEvent():
				if view >= 2 {
					r.totalVoteTime += time.Now().Sub(r.voteStart)
				}
				// measure round time
				now := time.Now()
				lasts := now.Sub(r.lastViewTime)
				r.totalRoundTime += lasts
				r.roundNo++
				r.lastViewTime = now
				r.eventChan <- view
				log.Debugf("[%v] the last view lasts %v milliseconds, current view: %v", r.ID(), lasts.Milliseconds(), view)
				break L
			case <-r.timer.C:
				r.Safety.ProcessLocalTmo(r.pm.GetCurView())
				break L
			}
		}
	}
}

// ListenCommittedBlocks listens committed blocks and forked blocks from the protocols
func (r *Replica) ListenCommittedBlocks() {
	for {
		select {
		case committedBlock := <-r.committedBlocks:
			r.processCommittedBlock(committedBlock)
		case forkedBlock := <-r.forkedBlocks:
			r.processForkedBlock(forkedBlock)
		}
	}
}

func (r *Replica) startSignal() {
	if !r.isStarted.Load() {
		r.startTime = time.Now()
		log.Debugf("[%v] is boosting", r.ID())
		r.isStarted.Store(true)
		r.start <- true
		r.Node.StartSignal()
	}
}

// Start starts event loop
func (r *Replica) Start() {
	go r.Node.Run()
	r.Node.RunPhalanx()
	// wait for the start signal
	<-r.start
	go r.ListenLocalEvent()
	go r.ListenCommittedBlocks()
	for r.isStarted.Load() {
		event := <-r.eventChan
		switch v := event.(type) {
		case types.View:
			r.processNewView(v)
		case blockchain.Block:
			startProcessTime := time.Now()
			r.totalProposeDuration += startProcessTime.Sub(v.Timestamp)
			_ = r.Safety.ProcessBlock(&v)
			r.totalProcessDuration += time.Now().Sub(startProcessTime)
			r.voteStart = time.Now()
			r.processedNo++
		case blockchain.Vote:
			startProcessTime := time.Now()
			r.Safety.ProcessVote(&v)
			processingDuration := time.Now().Sub(startProcessTime)
			r.totalVoteTime += processingDuration
			r.voteNo++
		case pacemaker.TMO:
			r.Safety.ProcessRemoteTmo(&v)
		case pCommonProto.ConsensusMessage:
			_ = r.Node.ReceiveConsensusMessage(&v)
		case pCommonProto.Command:
			r.Node.ReceiveCommand(&v)
		}
	}
}
