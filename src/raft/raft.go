package raft

//
// 这是 Raft 需要暴露给服务（或测试器）的 API 的概要。
// 具体查看每个函数下的注释以了解更多细节。
//
// rf = Make(...) 创建一个新的 Raft 服务端。
// rf.Start(command interface{}) (index, term, isleader) 开始对一个新的日志条目达成一致
// rf.GetState() (term, isLeader)
//   获取当前节点的 term，以及是否认为自己是 leader
// ApplyMsg
//   每当一个新的日志条目被提交时，Raft 节点需要通过相同服务上的 applyCh
//   将一个 ApplyMsg 发送到服务（或测试器）。

import (
	"6.5840/labgob"
	"6.5840/labrpc"
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// 这个是只给tester调的
// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = rf.state == Leader
	return term, isleader
}

// 在实现快照之前，传递 nil 作为 persister.Save() 的第二个参数。
// 实现快照后，传递当前快照（如果没有快照则传递 nil）。
func (rf *Raft) persist() { //2C才需要编写
	// 你的代码（用于 2C）。
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm) // 持久化任期
	e.Encode(rf.votedFor)    // 持久化votedFor
	e.Encode(rf.log)         // 持久化日志
	data := w.Bytes()
	go rf.persister.SaveRaftState(data)
	//DPrintf(100, "%v: persist rf.currentTerm=%v rf.voteFor=%v rf.log=%v\n", rf.SayMeL(), rf.currentTerm, rf.votedFor, rf.log)
}

// 恢复之前的持久化状态。
func (rf *Raft) readPersist() { //2C
	stateData := rf.persister.ReadRaftState()
	if stateData == nil || len(stateData) < 1 { // bootstrap without any state?
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// Your code here (2C).
	if stateData != nil && len(stateData) > 0 { // bootstrap without any state?
		r := bytes.NewBuffer(stateData)
		d := labgob.NewDecoder(r)
		rf.votedFor = 0 // in case labgob waring
		if d.Decode(&rf.currentTerm) != nil ||
			d.Decode(&rf.votedFor) != nil ||
			d.Decode(&rf.log) != nil {
			//   error...
			panic("readPersist失败")
		}
	}
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}
func (rf *Raft) sendRequestAppendEntries(isHeartbeat bool, server int, args *RequestAppendEntriesArgs, reply *RequestAppendEntriesReply) bool {
	var ok bool
	if isHeartbeat {
		ok = rf.peers[server].Call("Raft.HandleHeartbeatRPC", args, reply)
	} else {
		ok = rf.peers[server].Call("Raft.HandleAppendEntriesRPC", args, reply)
	}
	return ok
}

//Raft 的 Start(command interface{}) (index, term, isLeader)
//方法用于让客户端向 Raft 复制日志。
//若当前节点为 leader，应将该命令附加到自己的日志，并向 followers 发送
//AppendEntries RPC。
//若当前节点为 follower，则应拒绝该请求。则返回 false。
// 返回值包括：命令的索引、当前 term 以及是否是 leader。

func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true
	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	if rf.state != Leader {
		isLeader = false
		return index, term, isLeader
	}
	index = rf.log.LastLogIndex + 1
	// 开始发送AppendEntries rpc

	rf.log.appendL(Entry{term, command})
	rf.persist()
	DPrintf(111, "节点%d 调用了Start函数,commit是%d当前LastLogIndex是%d,Command是%T %v ", rf.me, rf.commitIndex, rf.log.LastLogIndex, command, command)
	go rf.StartAppendEntries(false)

	return index, term, isLeader
}

func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.applyHelper.Kill()

	rf.state = Follower
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead) // 这里的kill仅仅将对应的字段置为1
	return z == 1
}

// // the service says it has created a snapshot that has
// // all info up to and including index. this means the
// // service no longer needs the log through (and including)
// // that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

func (rf *Raft) StartAppendEntries(heart bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Leader {
		return
	}
	// 并行向其他节点发送心跳或者日志，让他们知道此刻已经有一个leader产生
	//DPrintf(111, "%v: detect the len of peers: %d", rf.SayMeL(), len(rf.peers))
	for i, _ := range rf.peers {
		if i == rf.me {
			continue
		}
		go rf.AppendEntries(i, heart)

	}
}

// 通知tester接收这个日志消息，然后供测试使用
func (rf *Raft) sendMsgToTester() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	for !rf.killed() {
		rf.applyCond.Wait()

		for rf.lastApplied+1 <= rf.commitIndex {
			i := rf.lastApplied + 1
			rf.lastApplied++
			if i < rf.log.FirstLogIndex {
				panic("error happening")
			}
			msg := ApplyMsg{
				CommandValid: true,
				Command:      rf.log.getOneEntry(i).Command,
				CommandIndex: i,
			}
			rf.applyHelper.tryApply(&msg)
		}
	}
}

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	// Your initialization code here (2A, 2B, 2C).
	rf.currentTerm = 0
	rf.votedFor = None
	rf.state = Follower //设置节点的初始状态为follower
	rf.resetElectionTimer()
	rf.heartbeatTimeout = heartbeatTimeout // 这个是固定的
	rf.log = NewLog()
	// initialize from state persisted before a crash
	rf.readPersist()
	fmt.Printf("当前节点%d的日志从持久化存储中读取的是", rf.me)
	fmt.Println(rf.log)
	rf.applyHelper = NewApplyHelper(applyCh, rf.lastApplied)
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.peerTrackers = make([]PeerTracker, len(rf.peers)) //对等节点追踪器
	rf.applyCond = sync.NewCond(&rf.mu)

	//Leader选举协程
	go rf.ticker()
	DPrintf(111, "第%d节点初始化完毕,任期是%d", rf.me, rf.currentTerm)
	go rf.sendMsgToTester() // 供config协程追踪日志以测试

	return rf
}

func (rf *Raft) ticker() {
	// 如果这个raft节点没有掉线,则一直保持活跃不下线状态（可以因为网络原因掉线，也可以tester主动让其掉线以便测试）
	for !rf.killed() {
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		switch state {
		case Follower:
			fallthrough
		case Candidate:
			if rf.pastElectionTimeout() {
				rf.StartElection()
			}

		case Leader:

			// 只有Leader节点才能发送心跳和日志给从节点
			isHeartbeat := false
			// 检测是需要发送单纯的心跳还是发送日志
			// 心跳定时器过期则发送心跳，否则发送日志
			if rf.pastHeartbeatTimeout() {
				isHeartbeat = true
				rf.resetHeartbeatTimer()
			}
			rf.StartAppendEntries(isHeartbeat)
		}
		time.Sleep(tickInterval)
	}
	DPrintf(111, "tim")
}
