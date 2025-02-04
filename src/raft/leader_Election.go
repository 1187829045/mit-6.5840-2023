package raft

func (rf *Raft) StartElection() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf(200, "节点%d选举超时开始选举任期是%d,当前的状态是%d", rf.me, rf.currentTerm+1, rf.state)
	rf.becomeCandidate()
	term := rf.currentTerm
	done := false
	votes := 1
	args := RequestVoteArgs{}
	args.Term = rf.currentTerm
	args.CandidateId = rf.me
	args.LastLogIndex = rf.log.LastLogIndex
	args.LastLogTerm = rf.getLastEntryTerm()
	defer rf.persist()
	for i, _ := range rf.peers {
		if rf.me == i {
			continue
		}
		// 开启协程去尝试拉选票
		go func(serverId int) {
			var reply RequestVoteReply
			ok := rf.sendRequestVote(serverId, &args, &reply)
			if !ok || !reply.VoteGranted {
				return
			}

			rf.mu.Lock()
			defer rf.mu.Unlock()

			if reply.Term < rf.currentTerm {
				return
			}
			// 统计票数
			votes++
			if done || votes <= len(rf.peers)/2 {
				return
			}
			if rf.state != Candidate || rf.currentTerm != term {
				return
			}
			rf.becomeLeader()
		}(i)
	}
}

// 定义一个心跳兼日志同步处理器，这个方法是Candidate和Follower节点的处理
func (rf *Raft) HandleHeartbeatRPC(args *RequestAppendEntriesArgs, reply *RequestAppendEntriesReply) {
	rf.mu.Lock() // 加接收心跳方的锁
	defer rf.mu.Unlock()
	reply.FollowerTerm = rf.currentTerm
	reply.Success = true
	// 旧任期的leader抛弃掉
	if args.LeaderTerm < rf.currentTerm {
		reply.Success = false
		return
	}
	rf.resetElectionTimer()
	// 需要转变自己的身份为Follower
	rf.state = Follower
	// 承认来者是个合法的新leader，则任期一定大于自己，此时需要设置votedFor为-1以及
	if args.LeaderTerm > rf.currentTerm {
		rf.votedFor = None
		rf.currentTerm = args.LeaderTerm
		reply.FollowerTerm = rf.currentTerm
	}
	// 重置自身的选举定时器，这样自己就不会重新发出选举需求（因为它在ticker函数中被阻塞住了）
}

// example RequestVoteRPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf(111, "节点%d收到%d选举的请求", rf.me, args.CandidateId)
	reply.VoteGranted = true // 默认设置响应体为投同意票状态
	reply.Term = rf.currentTerm
	//竞选leader的节点任期小于等于自己的任期，则反对票(为什么等于情况也反对票呢？因为candidate节点在发送requestVote rpc之前会将自己的term+1)
	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
		rf.persist()
		return
	}
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = None
		rf.state = Follower
		reply.Term = rf.currentTerm
	}

	// candidate节点发送过来的日志索引以及任期必须大于等于自己的日志索引及任期
	update := false
	update = update || args.LastLogTerm > rf.getLastEntryTerm()
	update = update || args.LastLogTerm == rf.getLastEntryTerm() && args.LastLogIndex >= rf.log.LastLogIndex

	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && update {
		rf.votedFor = args.CandidateId
		rf.state = Follower
		rf.resetElectionTimer() //自己的票已经投出时就转为follower状态
		DPrintf(111, "节点%d同意%d成为leader", rf.me, args.CandidateId)
		rf.persist()
	} else {
		reply.VoteGranted = false
		DPrintf(111, "节点%d拒绝%d成为leader,rf.votedFor = %d", rf.me, args.CandidateId, rf.votedFor)
	}
}
