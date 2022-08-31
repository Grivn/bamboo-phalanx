package node

import (
	"github.com/Grivn/phalanx/common/protos"
	pCommonTypes "github.com/Grivn/phalanx/common/types"
	phalanx "github.com/Grivn/phalanx/core"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/socket"
)

// Node is the primary access point for every replica
// it includes networking, state machine and RESTful API server
type Node interface {
	RunPhalanx()
	phalanx.Provider

	socket.Socket
	//Database
	ID() identity.NodeID
	Run()
	Retry(r message.Transaction)
	Forward(id identity.NodeID, r message.Transaction)
	Register(m interface{}, f interface{})
	IsByz() bool
	StartSignal()
	CommitBlock()
	QueryNode() QueryMessage
}

type QueryMessage struct {
	TThroughput    float64
	Throughput     float64
	TLatency       float64
	Latency        float64
	AveBlockSize   float64
	AvePayloadSize float64
	AveRealBlock   float64
}

// node implements Node interface
type node struct {
	id identity.NodeID
	phalanx.Provider

	socket.Socket
	//Database
	MessageChan chan interface{}
	TxChan      chan interface{}
	handles     map[string]reflect.Value
	server      *http.Server
	isByz       bool
	totalTxn    int

	sync.RWMutex
	forwards map[string]*message.Transaction

	totalCommittedTx int
	firstTimeAnchor  time.Time

	intervalCommittedTx int
	throughputAnchor    time.Time

	totalLatency float64
	latencyCount int

	intervalLatency      float64
	intervalLatencyCount int

	totalInnerBlock  int
	totalPayloadSize int
	totalBlockSize   int
	totalRealBlock   int
}

// NewNode creates a new Node object from configuration
func NewNode(id identity.NodeID, isByz bool) Node {
	n := &node{
		id:     id,
		isByz:  isByz,
		Socket: socket.NewSocket(id, config.Configuration.Addrs),
		//Database:    NewDatabase(),
		MessageChan: make(chan interface{}, config.Configuration.ChanBufferSize),
		TxChan:      make(chan interface{}, config.Configuration.ChanBufferSize),
		handles:     make(map[string]reflect.Value),
		forwards:    make(map[string]*message.Transaction),
	}

	idNum := uint64(id.Node())

	isPhalanxByz := false
	if idNum <= uint64(config.GetConfig().PhalanxByzNo) {
		isPhalanxByz = true
	}

	count := len(config.Configuration.Addrs)

	conf := phalanx.Config{
		OLeader:     uint64(config.GetConfig().PhalanxOligarchyLeader),
		Byz:         isPhalanxByz,
		Duration:    time.Duration(config.GetConfig().PhalanxDurationLog) * time.Millisecond,
		CDuration:   time.Duration(config.GetConfig().PhalanxDurationCommand) * time.Millisecond,
		Interval:    config.GetConfig().PhalanxInterval,
		OpenLatency: config.GetConfig().PhalanxOpenLatency,
		N:           count,
		Multi:       config.GetConfig().PhalanxMulti,
		LogCount:    config.GetConfig().PhalanxLogCount,
		MemSize:     config.GetConfig().MemSize,
		CommandSize: config.GetConfig().BSize,
		Author:      idNum,
		Network:     n,
		Exec:        n,
		Logger:      n,
		Selected:    uint64(config.GetConfig().PhalanxSelectedPropose),
	}
	n.Provider = phalanx.NewPhalanxProvider(conf)

	return n
}

func (n *node) ID() identity.NodeID {
	return n.id
}

func (n *node) IsByz() bool {
	return n.isByz
}

func (n *node) Retry(r message.Transaction) {
	log.Debugf("node %v retry reqeust %v", n.id, r)
	n.MessageChan <- r
}

// Register a handle function for each message type
func (n *node) Register(m interface{}, f interface{}) {
	t := reflect.TypeOf(m)
	fn := reflect.ValueOf(f)

	if fn.Kind() != reflect.Func {
		panic("handle function is not func")
	}

	if fn.Type().In(0) != t {
		panic("func type is not t")
	}

	if fn.Kind() != reflect.Func || fn.Type().NumIn() != 1 || fn.Type().In(0) != t {
		panic("register handle function error")
	}
	n.handles[t.String()] = fn
}

// Run start and run the node
func (n *node) Run() {
	log.Infof("node %v start running", n.id)
	if len(n.handles) > 0 {
		go n.handle()
		go n.recv()
		go n.txn()
	}
	n.http()
}

func (n *node) RunPhalanx() {
	n.Provider.Run()
}

func (n *node) txn() {
	for {
		tx := <-n.TxChan
		v := reflect.ValueOf(tx)
		name := v.Type().String()
		f, exists := n.handles[name]
		if !exists {
			log.Fatalf("no registered handle function for message type %v", name)
		}
		f.Call([]reflect.Value{v})
	}
}

//recv receives messages from socket and pass to message channel
func (n *node) recv() {
	for {
		m := n.Recv()
		if n.isByz && config.GetConfig().Strategy == "silence" {
			// perform silence attack
			continue
		}
		switch m := m.(type) {
		case message.Transaction:
			m.C = make(chan message.TransactionReply, 1)
			n.TxChan <- m
			continue

		case message.TransactionReply:
			n.RLock()
			r := n.forwards[m.Command.String()]
			log.Debugf("node %v received reply %v", n.id, m)
			n.RUnlock()
			r.Reply(m)
			continue
		}
		n.MessageChan <- m
	}
}

// handle receives messages from message channel and calls handle function using refection
func (n *node) handle() {
	for {
		msg := <-n.MessageChan
		v := reflect.ValueOf(msg)
		name := v.Type().String()
		f, exists := n.handles[name]
		if !exists {
			log.Fatalf("no registered handle function for message type %v", name)
		}
		f.Call([]reflect.Value{v})
	}
}

func (n *node) Forward(id identity.NodeID, m message.Transaction) {
	log.Debugf("Node %v forwarding %v to %s", n.ID(), m, id)
	m.NodeID = n.id
	n.Lock()
	n.forwards[m.Command.String()] = &m
	n.Unlock()
	n.Send(id, m)
}

func (n *node) StartSignal() {
	n.throughputAnchor = time.Now()
	n.firstTimeAnchor = time.Now()
}

func (n *node) CommitBlock() {
	n.totalInnerBlock++
}

func (n *node) QueryNode() QueryMessage {

	// calculate throughput and latency.
	totalThroughput := float64(n.totalCommittedTx) / time.Now().Sub(n.firstTimeAnchor).Seconds()
	throughput := float64(n.intervalCommittedTx) / time.Now().Sub(n.throughputAnchor).Seconds()
	totalLatency := n.totalLatency / float64(n.latencyCount)
	latency := n.intervalLatency / float64(n.intervalLatencyCount)

	// reset throughput info.
	n.intervalCommittedTx = 0
	n.throughputAnchor = time.Now()

	// reset interval latency.
	n.intervalLatency = 0
	n.intervalLatencyCount = 0

	// block size
	aveBlockSize := float64(n.totalBlockSize) / float64(n.totalRealBlock)

	// command size
	avePayloadSize := float64(n.totalPayloadSize) / float64(n.totalRealBlock)

	// committed block
	aveRealBlock := float64(n.totalRealBlock) / float64(n.totalInnerBlock)

	//n.totalBlockSize = 0
	//n.totalPayloadSize = 0
	//n.totalInnerBlock = 0
	//n.totalRealBlock = 0

	return QueryMessage{
		TThroughput:    totalThroughput,
		Throughput:     throughput,
		TLatency:       totalLatency,
		Latency:        latency,
		AveBlockSize:   aveBlockSize,
		AvePayloadSize: avePayloadSize,
		AveRealBlock:   aveRealBlock,
	}
}

//==================================================================================
//                              phalanx service
//==================================================================================

func (n *node) CommandExecution(block pCommonTypes.InnerBlock, seqNo uint64) {
	command := block.Command

	log.Infof("[%v] the block is committed, No. of transactions: %v, id: %d", n.ID(), len(command.Content), seqNo)

	for _, tx := range command.Content {
		// add the total committed tx for throughput.
		n.totalCommittedTx++
		n.intervalCommittedTx++

		// calculate latency for current transaction.
		n.totalLatency += pCommonTypes.NanoToSecond(time.Now().UnixNano()-tx.Timestamp) * 1000
		n.intervalLatency += pCommonTypes.NanoToSecond(time.Now().UnixNano()-tx.Timestamp) * 1000
		n.latencyCount++
		n.intervalLatencyCount++

		// calculate block size
		n.totalBlockSize++

		// calculate command size
		n.totalPayloadSize += len(tx.Payload)
	}
	n.totalRealBlock++
}

func (n *node) BroadcastCommand(command *protos.Command) {
	go n.Socket.Broadcast(*command)
	go n.ReceiveCommand(command)
}

func (n *node) BroadcastPCM(message *protos.ConsensusMessage) {
	go n.Socket.Broadcast(*message)
	go n.ReceiveConsensusMessage(message)
}

func (n *node) UnicastPCM(message *protos.ConsensusMessage) {
	if message.To == uint64(n.id.Node()) {
		go n.ReceiveConsensusMessage(message)
		return
	}
	go n.Send(identity.NewNodeID(int(message.To)), message)
}

func (n *node) Debug(v ...interface{}) {
	log.Debug(v...)
}
func (n *node) Debugf(format string, v ...interface{}) {
	log.Debugf(format, v...)
}

func (n *node) Info(v ...interface{}) {
	log.Info(v...)
}
func (n *node) Infof(format string, v ...interface{}) {
	log.Infof(format, v...)
}

func (n *node) Error(v ...interface{}) {
	log.Error(v...)
}
func (n *node) Errorf(format string, v ...interface{}) {
	log.Errorf(format, v...)
}
