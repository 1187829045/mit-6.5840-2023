package raft

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
//这是leader应用快照以及发送快照给所有从节点的入口函数，该函数由tester 测试程序调用， tester负责将快照好的数据传递给leader主机，
//leader更新完自己的日志FirstLogIndex 信息以及对快照进行持久化之后就会将快照数据传递给所有从节点的。

// 两个参数，一个是日志的index，一个是持久化后的snapshot字节数组, 这里的index其实就是lastIncludedIndex
// 这个方法应该就是发生故障重启后进行快照恢复的执行过程

func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	//leader节点收到快照后的步骤大概是
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Leader {
		return
	}
	DPrintf(111, "这是 Snapshot")
	if rf.log.FirstLogIndex <= index {
		//更新快照信息,lastIncludeIndex,lastIncludeTerm以及snapshot
		rf.snapshot = snapshot
		rf.snapshotLastIncludeIndex = index
		rf.snapshotLastIncludeTerm = rf.getEntryTerm(index)
		newFirstLogIndex := index + 1
		//更新日志的起始尾索引，并且截断或者清空日志
		if newFirstLogIndex <= rf.log.LastLogIndex {
			rf.log.Entries = rf.log.Entries[newFirstLogIndex-rf.log.FirstLogIndex:]
		} else {
			rf.log.LastLogIndex = newFirstLogIndex - 1
			rf.log.Entries = make([]Entry, 0)
		}
		//更新日志相关的参数commitIndex，状态机相关的lastApplied
		rf.log.FirstLogIndex = newFirstLogIndex
		rf.commitIndex = max(rf.commitIndex, index)
		rf.lastApplied = max(rf.lastApplied, index)
		//持久化快照和日志
		rf.persist()
		//将快照发送给所有从节点
		for i := 0; i < len(rf.peers); i++ {
			if i == rf.me {
				continue
			}
			go rf.InstallSnapshot(i)
		}
	}

}
func (rf *Raft) RequestInstallSnapshot(args *RequestInstallSnapShotArgs, reply *RequestInstallSnapShotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm //忘记加了
	if args.Term < rf.currentTerm {
		DPrintf(111, "节点%d的任期比请求节点%d任期大返回", rf.me, args.LeaderId)
		return
	}
	rf.state = Follower
	rf.resetElectionTimer()
	if args.Term > rf.currentTerm {
		rf.votedFor = -1
		rf.currentTerm = args.Term
		reply.Term = rf.currentTerm
	}
	DPrintf(111, "节点%d接受节点%d的快照,当前任期是%d", rf.me, args.LeaderId, rf.currentTerm)
	defer rf.persist()
	if args.LastIncludeIndex > rf.snapshotLastIncludeIndex {
		rf.snapshot = args.Snapshot
		rf.snapshotLastIncludeIndex = args.LastIncludeIndex
		rf.snapshotLastIncludeTerm = args.LastIncludeTerm
		if args.LastIncludeIndex >= rf.log.LastLogIndex {
			rf.log.Entries = make([]Entry, 0)
			rf.log.LastLogIndex = args.LastIncludeIndex
		} else {
			rf.log.Entries = rf.log.Entries[rf.log.getRealIndex(args.LastIncludeIndex+1):]
		}
		rf.log.FirstLogIndex = args.LastIncludeIndex + 1

		if args.LastIncludeIndex > rf.lastApplied {
			msg := &ApplyMsg{
				SnapshotValid: true,
				Snapshot:      rf.snapshot,
				SnapshotTerm:  rf.snapshotLastIncludeTerm,
				SnapshotIndex: rf.snapshotLastIncludeIndex,
			}
			rf.applyHelper.tryApply(msg)
			rf.lastApplied = args.LastIncludeIndex
		}
		rf.commitIndex = max(rf.commitIndex, args.LastIncludeIndex)
	}
	DPrintf(111, "节点%d当前的快照最大索引%d,日志长度是%d", rf.me, rf.snapshotLastIncludeIndex, len(rf.log.Entries))
}

func (rf *Raft) InstallSnapshot(serverId int) {

	args := &RequestInstallSnapShotArgs{}
	reply := &RequestInstallSnapShotReply{}
	rf.mu.Lock()
	if rf.state != Leader {
		DPrintf(99, "节点%d状态已变，不是leader节点，无法发送快照", rf.me)
		rf.mu.Unlock()
		return
	}
	DPrintf(111, "节点%d准备向节点%d发送快照", rf.me, serverId)
	args.Term = rf.currentTerm
	args.LeaderId = rf.me
	args.LastIncludeIndex = rf.snapshotLastIncludeIndex
	args.LastIncludeTerm = rf.snapshotLastIncludeTerm
	args.Snapshot = rf.snapshot
	rf.mu.Unlock()
	ok := rf.sendRequestInstallSnapshot(serverId, args, reply)
	if !ok {
		//DPrintf(111, "sendRequestInstallSnapshot函数调用失败")
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != Leader {
		//DPrintf(111, "节点%d因为不是leader，放弃处理%d的快照响应", rf.me, serverId)
		return
	}
	if reply.Term < rf.currentTerm {
		DPrintf(111, "节点%d因为是旧的快照响应，放弃处理%d的快照响应, 旧响应的任期是%d", rf.me, serverId, reply.Term)
		return
	}
	if reply.Term > rf.currentTerm {
		rf.votedFor = -1
		rf.state = Follower
		rf.currentTerm = reply.Term
		rf.persist()
		return
	}
	rf.peerTrackers[serverId].nextIndex = args.LastIncludeIndex + 1
	rf.peerTrackers[serverId].matchIndex = args.LastIncludeIndex
	DPrintf(111, "节点%d更新节点%d的nextIndex为%d, matchIndex为%d", rf.me, serverId, rf.peerTrackers[serverId].nextIndex, args.LastIncludeIndex)

	rf.tryCommitL(rf.peerTrackers[serverId].matchIndex)

}

func (rf *Raft) sendRequestInstallSnapshot(server int, args *RequestInstallSnapShotArgs, reply *RequestInstallSnapShotReply) bool {
	ok := rf.peers[server].Call("Raft.RequestInstallSnapshot", args, reply)
	return ok
}
