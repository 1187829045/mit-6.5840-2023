package raft

func (rf *Raft) StartElection() {
	//1.增加自己的任期
	//2.为自己投票，更新状态为候选者
	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf("节点%d选举超时开始选举任期是%d,当前的状态是%d", rf.me, rf.currentTerm+1, rf.state)
	if rf.state == Leader {
		// 如果是leader，则应该不应该进行选举
		DPrintf("节点%d是Leader不再选举", rf.me)
		return
	}
	rf.becomeCandidate()
	term := rf.currentTerm
	done := false
	votes := 1
	args := RequestVoteArgs{}
	args.Term = rf.currentTerm
	args.CandidateId = rf.me
	args.LastLogIndex = rf.log.LastLogIndex
	args.LastLogTerm = rf.getLastEntryTerm()
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(serverId int) { // 为什么用 携程启动 没有死锁发生
			var reply RequestVoteReply
			ok := rf.sendRequestVote(serverId, &args, &reply)
			if !ok || !reply.VoteGranted {
				DPrintf("调用函数失败")
				return
			}
			rf.mu.Lock()
			defer rf.mu.Unlock()
			if reply.Term < rf.currentTerm {
				DPrintf(" reply.Term < rf.currentTerm")
				return
			}
			// 统计票数
			votes++
			if done || votes <= len(rf.peers)/2 {
				// 在成为leader之前如果投票数不足需要继续收集选票
				// 同时在成为leader的那一刻，就不需要管剩余节点的响应了，因为已经具备成为leader的条件
				return
			}
			done = true
			if rf.state != Candidate || rf.currentTerm != term {
				return
			}
			rf.becomeLeader()
		}(i)
	}
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf("节点%d向节点%d发起成为leader请求，args.Term ： %d,rf.currentTerm:%d,args.LastLogTerm :%d,当前节点的最后日志任期:%d",
		args.CandidateId, rf.me, args.Term, rf.currentTerm, args.LastLogTerm, rf.getLastEntryTerm())
	reply.VoteGranted = true // 默认设置响应体为投同意票状态
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		DPrintf("节点%d比请求节点%d任期大于返回false", rf.me, args.CandidateId)
		reply.VoteGranted = false
		return
	}
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		reply.Term = rf.currentTerm
		rf.votedFor = NoVote //需要设置为noVote
		rf.state = Follower  // 这里暂时添加看是否正确
	}
	// candidate节点发送过来的日志索引以及任期必须大于等于自己的日志索引及任期
	update := false
	update = update || args.LastLogTerm > rf.getLastEntryTerm()
	update = update || args.LastLogTerm == rf.getLastEntryTerm() && args.LastLogIndex >= rf.log.LastLogIndex
	// 去掉rf.votedFor == rf.me的情况,会更快,避免了候选者变成跟随者,跟随者变成候选者的来回转换
	if (rf.votedFor == NoVote || rf.votedFor == args.CandidateId) && update {
		//竞选任期大于自身任期，则更新自身任期，并转为follower
		rf.votedFor = args.CandidateId
		rf.state = Follower
		DPrintf("节点%d同意%d成为leader", rf.me, args.CandidateId)
		rf.ResetElectionTime()
	} else {
		reply.VoteGranted = false
		DPrintf("节点%d拒绝%d成为leader,rf.votedFor = %d", rf.me, args.CandidateId, rf.votedFor)
	}
}
