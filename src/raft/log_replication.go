package raft

// nextIndex收敛速度优化：nextIndex跳跃算法，需搭配HandleAppendEntriesRPC2方法使用
func (rf *Raft) AppendEntries(targetServerId int, heart bool) {
	rf.mu.Lock()
	rf.resetElectionTimer()
	rf.mu.Unlock()
	if heart {
		reply := RequestAppendEntriesReply{}
		args := RequestAppendEntriesArgs{}
		rf.mu.Lock()
		if rf.state != Leader {
			rf.mu.Unlock()
			return
		}
		args.LeaderTerm = rf.currentTerm
		DPrintf(111, "节点%d向节点%d发送心跳", rf.me, targetServerId)
		rf.mu.Unlock()
		rf.sendRequestAppendEntries(true, targetServerId, &args, &reply)
		return
	} else {
		args := RequestAppendEntriesArgs{}
		reply := RequestAppendEntriesReply{}

		rf.mu.Lock()
		DPrintf(111, "节点%d向节点%d发送日志", rf.me, targetServerId)
		if rf.state != Leader {
			rf.mu.Unlock()
			return
		}
		args.PrevLogIndex = min(rf.log.LastLogIndex, rf.peerTrackers[targetServerId].nextIndex-1)
		if args.PrevLogIndex+1 < rf.log.FirstLogIndex {
			DPrintf(111, "节点%d日志匹配索引为%d更新速度太慢，节点%d准备发送快照", targetServerId, args.PrevLogIndex, rf.me)
			go rf.InstallSnapshot(targetServerId)
			rf.mu.Unlock()
			return
		}
		args.LeaderTerm = rf.currentTerm
		args.LeaderId = rf.me
		args.LeaderCommit = rf.commitIndex
		args.PrevLogTerm = rf.getEntryTerm(args.PrevLogIndex)
		args.Entries = rf.log.getAppendEntries(args.PrevLogIndex + 1)
		rf.mu.Unlock()

		ok := rf.sendRequestAppendEntries(false, targetServerId, &args, &reply)
		if !ok {
			DPrintf(111, "sendRequestAppendEntries函数调用失败")
			return
		}
		rf.mu.Lock()
		defer rf.mu.Unlock()
		if rf.state != Leader {
			return
		}
		// 丢掉旧rpc响应
		if reply.FollowerTerm < rf.currentTerm {
			return
		}
		if reply.FollowerTerm > rf.currentTerm {
			rf.state = Follower
			rf.votedFor = None
			rf.currentTerm = reply.FollowerTerm
			rf.persist()
			return
		}

		if reply.Success {
			rf.peerTrackers[targetServerId].nextIndex = args.PrevLogIndex + len(args.Entries) + 1
			rf.peerTrackers[targetServerId].matchIndex = args.PrevLogIndex + len(args.Entries)
			rf.tryCommitL(rf.peerTrackers[targetServerId].matchIndex)
			return
		}
		if rf.log.empty() {
			DPrintf(111, "日志被快照清空，发送给%d快照", targetServerId)
			go rf.InstallSnapshot(targetServerId)
			return
		}
		if reply.PrevLogIndex+1 < rf.log.FirstLogIndex {
			DPrintf(111, "节点%d日志匹配索引为%d更新速度太慢，节点%d准备发送快照,快照的最大索引是%d",
				targetServerId, args.PrevLogIndex, rf.me, rf.snapshotLastIncludeIndex)
			go rf.InstallSnapshot(targetServerId)
			return
		}
		if reply.PrevLogIndex > rf.log.LastLogIndex {
			rf.peerTrackers[targetServerId].nextIndex = rf.log.LastLogIndex + 1
		} else if rf.getEntryTerm(reply.PrevLogIndex) == reply.PrevLogTerm {
			rf.peerTrackers[targetServerId].nextIndex = reply.PrevLogIndex + 1
		} else {
			PrevIndex := reply.PrevLogIndex
			for PrevIndex >= rf.log.FirstLogIndex && rf.getEntryTerm(PrevIndex) == rf.getEntryTerm(reply.PrevLogIndex) {
				PrevIndex--
			}
			rf.peerTrackers[targetServerId].nextIndex = PrevIndex + 1
		}
	}
}

func (rf *Raft) HandleAppendEntriesRPC(args *RequestAppendEntriesArgs, reply *RequestAppendEntriesReply) {
	rf.mu.Lock() // 加接收日志方的锁
	defer rf.mu.Unlock()
	reply.FollowerTerm = rf.currentTerm
	reply.Success = true
	// 旧任期的leader抛弃掉
	if args.LeaderTerm < rf.currentTerm {
		reply.Success = false
		DPrintf(111, "节点%d拒绝%d发送来的日志", rf.me, args.LeaderId)
		return
	}
	DPrintf(111, "节点%d接收%d发送来的日志", rf.me, args.LeaderId)
	rf.resetElectionTimer()
	rf.state = Follower

	if args.LeaderTerm > rf.currentTerm {
		rf.votedFor = None
		rf.currentTerm = args.LeaderTerm
		reply.FollowerTerm = rf.currentTerm
	}
	defer rf.persist()
	if args.PrevLogIndex+1 < rf.log.FirstLogIndex || args.PrevLogIndex > rf.log.LastLogIndex {
		reply.FollowerTerm = rf.currentTerm
		reply.Success = false
		reply.PrevLogIndex = rf.log.LastLogIndex
		reply.PrevLogTerm = rf.getLastEntryTerm()
	} else if rf.getEntryTerm(args.PrevLogIndex) == args.PrevLogTerm {
		ok := true
		for i, entry := range args.Entries {
			index := args.PrevLogIndex + 1 + i
			if index > rf.log.LastLogIndex {
				rf.log.appendL(entry)
			} else if rf.log.getOneEntry(index).Term != entry.Term {
				// 采用覆盖写的方式
				ok = false
				*rf.log.getOneEntry(index) = entry
			}
		}
		if !ok {
			rf.log.LastLogIndex = args.PrevLogIndex + len(args.Entries)
		}
		if args.LeaderCommit > rf.commitIndex {
			if args.LeaderCommit < rf.log.LastLogIndex {
				rf.commitIndex = args.LeaderCommit
			} else {
				rf.commitIndex = rf.log.LastLogIndex
			}
			rf.applyCond.Broadcast()
		}
		reply.FollowerTerm = rf.currentTerm
		reply.Success = true
		reply.PrevLogIndex = rf.log.LastLogIndex
		reply.PrevLogTerm = rf.getLastEntryTerm()
	} else {
		prevIndex := args.PrevLogIndex
		for prevIndex >= rf.log.FirstLogIndex && rf.getEntryTerm(prevIndex) == rf.log.getOneEntry(args.PrevLogIndex).Term {
			prevIndex--
		}
		reply.FollowerTerm = rf.currentTerm
		reply.Success = false
		if prevIndex >= rf.log.FirstLogIndex {
			reply.PrevLogIndex = prevIndex
			reply.PrevLogTerm = rf.getEntryTerm(prevIndex)
		}

	}

}

// 主节点对日志进行提交，其条件是多余一半的从节点的commitIndex>=leader节点当前提交的commitIndex
func (rf *Raft) tryCommitL(matchIndex int) {
	if matchIndex <= rf.commitIndex {
		// 首先matchIndex应该是大于leader节点的commitIndex才能提交，因为commitIndex及其之前的不需要更新
		DPrintf(111, "matchIndex <= rf.commitIndex")
		return
	}
	// 越界的也不能提交
	if matchIndex > rf.log.LastLogIndex {
		DPrintf(111, "matchIndex > rf.log.LastLogIndex")
		return
	}
	// 提交的必须本任期内从客户端收到的日志
	if rf.getEntryTerm(matchIndex) != rf.currentTerm {
		DPrintf(111, "rf.getEntryTerm(matchIndex) != rf.currentTerm")
		return
	}

	// 计算所有已经正确匹配该matchIndex的从节点的票数
	cnt := 1 //自动计算上leader节点的一票
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}
		if matchIndex <= rf.peerTrackers[i].matchIndex {
			cnt++
		}
	}
	// 超过半数就提交
	if cnt > len(rf.peers)/2 {
		rf.commitIndex = matchIndex
		if rf.commitIndex > rf.log.LastLogIndex {
			panic("")
		}
		DPrintf(111, "对日志进行提交")
		rf.applyCond.Broadcast() // 通知对应的applier协程将日志放到状态机上验证
	} else {
		DPrintf(111, "未超过半数节点在此索引上的日志相等，拒绝提交....")
	}
}
