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
	"math"
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

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf("节点%d收到节点%d的选票，args.Term ： %d,rf.currentTerm:%d,"+
		"args.LastLogTerm :%d,rf.log[len(rf.log)-1].Term:%d",
		rf.me, args.CandidateId, args.Term, rf.currentTerm,
		args.LastLogTerm, rf.log[len(rf.log)-1].Term)

	if args.Term <= rf.currentTerm {
		DPrintf("节点%d比请求节点任期大于等于返回false", rf.me)
		reply.VoteGranted = false
		reply.Term = rf.currentTerm
	} else {
		//DPrintf("准备投票")
		rf.currentTerm = args.Term
		reply.VoteGranted = true
		reply.Term = rf.currentTerm
		rf.votedFor = args.CandidateId
		rf.ResetHeartbeatTime()
		rf.ResetElectionTime()
		rf.state = Follower
		//DPrintf("节点%d比请求节点任期小，并且没有给他人投票，返回true", rf.me)
	}
}

//如果 term < currentTerm，返回 false。
//如果日志中 prevLogIndex 的任期号与 prevLogTerm 不匹配，返回 false。
//如果新条目与已有条目冲突（相同索引但不同任期号），删除该条目及后续条目。
//附加新的日志条目。
//如果 leaderCommit > commitIndex，更新 commitIndex。

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf("节点%d接收到节点%d心跳，我的状态是%d", rf.me, args.LeaderId, rf.state)
	// 如果领导者的任期小于接收者的当前任期，返回 false
	if args.Term < rf.currentTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	} else if len(rf.log) <= args.PrevLogIndex || rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		//log.Println("len(rf.log)", len(rf.log), "args.PrevLogIndex", args.PrevLogIndex)
		reply.Success = false
		reply.Term = rf.currentTerm
	} else {
		reply.Success = true
		rf.currentTerm = args.Term
		reply.Term = rf.currentTerm
		rf.ResetHeartbeatTime()
		rf.ResetElectionTime()
		rf.state = Follower
		DPrintf("节点%d接收到节点%d心跳，我的状态改变为%d", rf.me, args.LeaderId, rf.state)
		if len(args.Entries) > 0 {
			rf.log = append(rf.log, args.Entries...)
		}
		if args.LeaderCommit > rf.commitIndex {
			rf.commitIndex = int(math.Min(float64(args.LeaderCommit), float64(len(rf.log)-1)))
		}
	}
}

// 示例代码：发送一个 RequestVote RPC 给某个服务器。
// server 是目标服务器在 rf.peers[] 中的索引。
// args 是请求参数。
// *reply 是用来接收回复的，调用者应该传递 &reply。

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	//DPrintf("ready to call RequestVote Method...")
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}
func (rf *Raft) sendRequestAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// 服务（如 K/V 服务）希望开始对下一个命令达成一致。
// 如果此节点不是 leader，则返回 false。
// 如果是 leader，则立即启动共识过程并返回。
// 返回值包括：命令的索引、当前 term 以及是否是 leader。

func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// 你的代码（用于 2B）。

	return index, term, isLeader
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

func (rf *Raft) StartAppendEntries() {
	// 所有节点共享同一份request参数
	args := AppendEntriesArgs{}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.ResetHeartbeatTime()
	args.Term = rf.currentTerm
	args.LeaderId = rf.me
	args.PrevLogTerm = rf.log[len(rf.log)-1].Term
	args.PrevLogIndex = len(rf.log) - 1
	args.LeaderCommit = rf.commitIndex
	args.Entries = []Entry{}
	reply := make([]AppendEntriesReply, len(rf.peers))

	// 并行向其他节点发送心跳，让他们知道此刻已经有一个leader产生
	for i, _ := range rf.peers {
		if i == rf.me {
			continue
		}
		go rf.sendRequestAppendEntries(i, &args, &reply[i])
	}
}
func (rf *Raft) StartElection() {

	//1.增加自己的任期
	//2.为自己投票，更新状态为候选者
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.currentTerm++
	DPrintf("节点%d开始发起选举,当前任期%d", rf.me, rf.currentTerm)
	rf.votedFor = rf.me
	Term := rf.currentTerm
	rf.state = Candidate
	rf.ResetElectionTime()
	rf.ResetHeartbeatTime()
	votes := 1
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(peer int) {
			args := &RequestVoteArgs{
				Term:         rf.currentTerm,
				CandidateId:  rf.me,
				LastLogIndex: len(rf.log) - 1,
				LastLogTerm:  rf.log[len(rf.log)-1].Term,
			}
			var reply RequestVoteReply
			if rf.sendRequestVote(peer, args, &reply) {
				if reply.VoteGranted {
					votes++
					if rf.state == Candidate && votes >= (len(rf.peers)+1)/2 && Term == rf.currentTerm {
						rf.state = Leader
						DPrintf("当前%d节点成为leader，并发送心跳", rf.me)
						go rf.StartAppendEntries() //发起心跳告知我是领导者
					}
				}
			}
		}(i)
	}
}

// 1.这个后台线程应该不断检查是否有超时
// 2.如果是领导者，心跳机制超时，应该再次发起心跳，并重置心跳发起的时间
// 3.如果是候选者，应该检查的是选举超时时间是否达到，达到应该发去选举，重置选举时间
// 4.如果是跟随着应该看心跳是否超时，超时应该将自己设置为候选者
func (rf *Raft) ticker() {
	for rf.killed() == false {

		//DPrintf( "正在检查是否超时.......")
		// 你的代码（用于 2A）
		// 检查是否应该启动领导选举。
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		//rf.mu.Lock()
		switch state {
		case Follower:

			fallthrough
		case Candidate:
			if rf.IsElectionTimeout() { //#A
				rf.StartElection()
			}
		case Leader:
			if rf.IsHeartTimeOut() {
				DPrintf("节点%d当前状态是Leader,且心跳超时", rf.me)
				rf.StartAppendEntries()
			}
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
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.state = Candidate
	rf.currentTerm = 0
	rf.votedFor = NoVote
	rf.heartbeatTimeout = HeartTimeOut
	rf.ResetElectionTime()
	rf.peerTrackers = make([]PeerTracker, len(peers))
	rf.log = make([]Entry, 0)
	rf.log = append(rf.log, Entry{
		Term: 0,
	})

	// 从持久化状态恢复
	rf.readPersist(persister.ReadRaftState())
	DPrintf("当前是第 %d 节点初始化完成", rf.me)
	// 启动 ticker goroutine 以启动选举
	go rf.ticker()

	return rf
}
