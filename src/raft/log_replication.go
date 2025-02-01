package raft

// 应该再加入参数 commitIndex 这样从节点也去提交，还没实现
// 不是心跳时候需要传入的entry不应该作为参数传递,还没实现,明天观察测试用例以及别人的实现
func (rf *Raft) StartAppendEntries(heart bool) {
	// 并行向其他节点发送心跳或者日志，让他们知道此刻已经有一个leader产生
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if heart == false {
		DPrintf("节点%d开始发起日志复制", rf.me)
	} else {
		DPrintf("节点%d开始发起心跳", rf.me)
	}
	if rf.state != Leader {
		return
	}
	rf.ResetElectionTime()
	for i, _ := range rf.peers {
		if i == rf.me {
			continue
		}
		go rf.AppendEntries(i, heart)
	}
}

//如果 term < currentTerm，返回 false。
//如果日志中 prevLogIndex 的任期号与 prevLogTerm 不匹配(分别讨论大于等于小于三种情况)，返回 false（或者true）。
//如果新条目与已有条目冲突（相同索引但不同任期号，大于小于分别讨论），删除该条目及后续条目。
//附加新的日志条目。
//如果 leaderCommit > commitIndex，更新 commitIndex。

func (rf *Raft) AppendEntries(targetServerId int, heart bool) {

	if heart {
		rf.ResetHeartbeatTime()
		rf.mu.Lock() //也取消defer unlock
		if rf.state != Leader {
			//DPrintf("当前节点%d不是Leader", rf.me)
			rf.mu.Unlock()
			return
		}
		reply := AppendEntriesReply{}
		args := AppendEntriesArgs{}
		args.Term = rf.currentTerm
		args.LeaderId = rf.me
		DPrintf("节点%d向节点%d发起心跳", rf.me, targetServerId)
		rf.mu.Unlock()
		ok := rf.sendRequestAppendEntries(true, targetServerId, &args, &reply)
		if !ok {
			return
		}
		if reply.Success {
			// 返回成功则说明接收了心跳
			return
		}
		rf.mu.Lock()
		if rf.state != Leader {
			rf.mu.Unlock()
			return
		}
		if reply.Term < rf.currentTerm {
			rf.mu.Unlock()
			return
		}
		// 拒绝接收心跳，则可能是因为任期导致的
		if reply.Term > rf.currentTerm {
			rf.votedFor = NoVote
			rf.state = Follower
			rf.currentTerm = reply.Term
		}
		rf.mu.Unlock()
		return
	} else {
		//取消defer lock大大缩短时间
		rf.mu.Lock()
		if rf.state != Leader {
			rf.mu.Unlock()
			return
		}
		args := AppendEntriesArgs{}
		args.PrevLogIndex = min(rf.log.LastLogIndex, rf.peerTrackers[targetServerId].nextIndex-1)
		args.Term = rf.currentTerm
		args.LeaderId = rf.me
		args.LeaderCommit = rf.commitIndex
		args.PrevLogTerm = rf.getEntryTerm(args.PrevLogIndex)
		args.Entries = rf.log.getAppendEntries(args.PrevLogIndex + 1)
		rf.mu.Unlock()
		reply := AppendEntriesReply{}
		DPrintf("节点%d向节点%d发起日志复制", rf.me, targetServerId)
		ok := rf.sendRequestAppendEntries(false, targetServerId, &args, &reply)
		if !ok {
			//DPrintf("调用函数失败")
			return
		}
		rf.mu.Lock()
		defer rf.mu.Unlock()
		if rf.state != Leader {
			//DPrintf("当前不是leader节点")
			return
		}
		// 丢弃旧的rpc响应
		if reply.Term < rf.currentTerm {
			return
		}
		if reply.Success {
			rf.peerTrackers[targetServerId].nextIndex = args.PrevLogIndex + len(args.Entries) + 1
			rf.peerTrackers[targetServerId].matchIndex = args.PrevLogIndex + len(args.Entries)
			rf.tryCommitL(rf.peerTrackers[targetServerId].matchIndex)
			return
		}
		//reply.Success is false,分别讨论
		if reply.Term > rf.currentTerm {
			rf.state = Follower
			rf.currentTerm = reply.Term
			rf.votedFor = NoVote
			return
		}
		//两种情况，一个是prelogIndex>lastlogIndex 或者prelogIndex  这个索引的人气不一致
		rf.peerTrackers[targetServerId].nextIndex = reply.PrevLogIndex + 1
	}

}

// 定义一个心跳兼日志同步处理器，这个方法是Candidate和Follower节点的处理
func (rf *Raft) HandleHeartbeatRPC(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock() // 加接收心跳方的锁
	defer rf.mu.Unlock()
	DPrintf("%d节点接受来自%d节点的心跳", rf.me, args.LeaderId)
	reply.Term = rf.currentTerm
	reply.Success = true
	// 旧任期的leader抛弃掉
	if args.Term < rf.currentTerm {
		DPrintf("%d节点丢弃来自%d节点的心跳", rf.me, args.LeaderId)
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}
	// 需要转变自己的身份为Follower
	rf.ResetElectionTime()
	rf.state = Follower
	rf.votedFor = args.LeaderId

	// 承认来者是个合法的新leader，则任期一定大于自己，此时需要设置votedFor为-1以及
	if args.Term > rf.currentTerm {
		rf.votedFor = NoVote
		rf.currentTerm = args.Term
		reply.Term = rf.currentTerm

	}

}

// nextIndex收敛速度优化：nextIndex跳跃算法
// 定义一个心跳兼日志同步处理器，这个方法是Candidate和Follower节点的处理

func (rf *Raft) HandleAppendEntriesRPC(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.ResetElectionTime()
	//判断任期是否比自己大
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		reply.PrevLogTerm = rf.getLastEntryTerm()
		reply.PrevLogIndex = rf.log.LastLogIndex
		rf.state = Candidate
		return
	}
	rf.state = Follower // 需要转变自己的保持为Follower
	if args.Term > rf.currentTerm {
		rf.votedFor = NoVote // 调整votedFor为-1
		rf.currentTerm = args.Term
		reply.Term = rf.currentTerm
	}
	//判断PrevLogIndex 如果大于当前节点的最大索引
	if args.PrevLogIndex > rf.log.LastLogIndex {
		reply.Term = rf.currentTerm
		reply.Success = false
		reply.PrevLogIndex = rf.log.LastLogIndex
		reply.PrevLogTerm = rf.getLastEntryTerm()
		return
	}
	//当前节点的PrevLogIndex下标的任期与leader节点的任不一致
	if rf.getEntryTerm(args.PrevLogIndex) != args.PrevLogTerm {
		prevIndex := args.PrevLogIndex
		for prevIndex >= 0 && rf.getEntryTerm(prevIndex) == rf.getEntryTerm(args.PrevLogIndex) {
			prevIndex--
		}
		reply.Term = rf.currentTerm
		reply.Success = false
		if prevIndex >= 0 {
			reply.PrevLogIndex = prevIndex
			reply.PrevLogTerm = rf.getEntryTerm(prevIndex)
		}
		return
	}
	//当前节点的PrevLogIndex下标的任期与leader节点的任期一致,可以开启复制
	for i, entry := range args.Entries {
		index := args.PrevLogIndex + 1 + i
		if index > rf.log.LastLogIndex {
			rf.log.appendEntry(entry)
		} else if rf.getEntryTerm(index) != entry.Term {
			*rf.log.getOneEntry(index) = entry
		}
	}
	rf.log.LastLogIndex = max(args.PrevLogIndex+len(args.Entries), rf.log.LastLogIndex)
	if args.LeaderCommit > rf.commitIndex {
		if args.LeaderCommit < rf.log.LastLogIndex {
			rf.commitIndex = args.LeaderCommit
		} else {
			rf.commitIndex = rf.log.LastLogIndex
		}
		//DPrintf("开启广播")
		rf.applyCond.Broadcast()
	}
	reply.Term = rf.currentTerm
	reply.Success = true
	reply.PrevLogIndex = rf.log.LastLogIndex
	reply.PrevLogTerm = rf.getLastEntryTerm()
}

// 主节点对日志进行提交，其条件是多余一半的从节点的commitIndex>=leader节点当前提交的commitIndex
func (rf *Raft) tryCommitL(matchIndex int) {
	if matchIndex <= rf.commitIndex {
		// 首先matchIndex应该是大于leader节点的commitIndex才能提交，因为commitIndex及其之前的不需要更新
		DPrintf(" matchIndex <= rf.commitIndex")
		return
	}
	// 越界的也不能提交
	if matchIndex > rf.log.LastLogIndex {
		DPrintf("  matchIndex > rf.log.LastLogIndex")
		return
	}
	// 提交的必须本任期内从客户端收到的日志
	if rf.getEntryTerm(matchIndex) != rf.currentTerm {
		DPrintf(" rf.getEntryTerm(matchIndex) != rf.currentTerm")
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
	if cnt > len(rf.peers)/2 {
		rf.commitIndex = matchIndex
		if rf.commitIndex > rf.log.LastLogIndex {
			panic("")
		}
		DPrintf("对日志进行提交")
		rf.applyCond.Broadcast() // 通知对应的applier协程将日志放到状态机上验证
	} else {
		DPrintf("未超过半数节点在此索引上的日志相等，拒绝提交....")
	}
}
