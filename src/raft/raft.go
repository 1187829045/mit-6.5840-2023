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
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

// 返回当前 term 以及节点是否认为自己是 leader。

func (rf *Raft) GetState() (int, bool) {
	var term int
	var isleader bool
	// 你的代码（用于 2A）。
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
	// 示例：
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// 恢复之前的持久化状态。
func (rf *Raft) readPersist(data []byte) { //2C
	if data == nil || len(data) < 1 { // 没有任何状态时初始化
		return
	}
	// 你的代码（用于 2C）。
	// 示例：
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// 服务通知 Raft 创建了一个快照，包含了到 index 为止的所有信息。
// 这意味着服务不再需要从（包含）该索引到之前的日志。
// Raft 应尽可能修剪其日志。

func (rf *Raft) Snapshot(index int, snapshot []byte) { //2D
	// 你的代码（用于 2D）。

}

// 示例代码：发送一个 RequestVote RPC 给某个服务器。
// server 是目标服务器在 rf.peers[] 中的索引。
// args 是请求参数。
// *reply 是用来接收回复的，调用者应该传递 &reply。

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}
func (rf *Raft) sendRequestAppendEntries(isHeartbeat bool, server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
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

	// 你的代码（用于 2B）。
	if rf.state != Leader {
		isLeader = false
		return index, term, isLeader
	}
	rf.log.appendEntry(Entry{term, command})
	go rf.StartAppendEntries(false)
	return rf.log.LastLogIndex, rf.currentTerm, isLeader
}

// tester 不会停止 Raft 创建的 goroutine，但会调用 Kill() 方法。
// 你的代码可以使用 killed() 检查是否已被 Kill() 调用。
// 使用 atomic 避免锁。

func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// 可选：你的代码

}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// 1.这个后台线程应该不断检查是否有超时
// 2.如果是领导者，心跳机制超时，应该再次发起心跳，并重置心跳发起的时间
// 3.如果是候选者，应该检查的是选举超时时间是否达到，达到应该发去选举，重置选举时间
// 4.如果是跟随着应该看心跳是否超时，超时应该将自己设置为候选者
func (rf *Raft) ticker() {
	for rf.killed() == false {
		// 你的代码（用于 2A）
		// 检查是否应该启动领导选举。
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		switch state {
		case Follower:
			fallthrough
		case Candidate:
			if rf.IsElectionTimeout() {
				rf.StartElection()
			}
		case Leader:
			// 只有Leader节点才能发送心跳和日志给从节点
			isHeartbeat := false
			// 检测是需要发送单纯的心跳还是发送日志
			// 心跳定时器过期则发送心跳，否则发送日志
			if rf.IsHeartTimeOut() {
				isHeartbeat = true
				rf.ResetHeartbeatTime()
				rf.StartAppendEntries(isHeartbeat)
			}
			rf.StartAppendEntries(isHeartbeat)
		}

		// 随机暂停 50 到 350 毫秒之间。
		ms := 50
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

// 创建一个 Raft 节点。
// peers[] 包含所有 Raft 节点的端口。
// 当前节点的端口为 peers[me]。
// persister 用于保存和加载节点的持久化状态。
// applyCh 是服务或 tester 希望 Raft 发送 ApplyMsg 的 channel。

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {

	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	// 初始化代码（用于 2A, 2B, 2C）。
	rf.commitIndex = -1
	rf.lastApplied = -1
	rf.state = Candidate
	rf.currentTerm = 0
	rf.votedFor = NoVote
	rf.heartbeatTimeout = HeartTimeOut
	rf.ResetElectionTime()
	rf.peerTrackers = make([]PeerTracker, len(peers))
	rf.log = NewLog()
	// 从持久化状态恢复
	rf.readPersist(persister.ReadRaftState())
	// 启动 ticker goroutine 以启动选举
	go rf.ticker()
	DPrintf("第%d节点初始化完毕,任期是%d", rf.me, rf.currentTerm)
	return rf
}
