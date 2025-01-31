package raft

import (
	"math/rand"
	"time"
)

func (rf *Raft) IsHeartTimeOut() bool {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return time.Since(rf.lastHeartbeat) > rf.heartbeatTimeout
}

func (rf *Raft) ResetHeartbeatTime() {
	rf.lastHeartbeat = time.Now()
}

func (rf *Raft) IsElectionTimeout() bool {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	return time.Since(rf.lastElection) > rf.electionTimeout
}

func (rf *Raft) ResetElectionTime() {
	rf.lastElection = time.Now()
	electionTimeout := ElectionTimeoutBase + (rand.Int63() % ElectionTimeoutBase)
	rf.electionTimeout = time.Duration(electionTimeout) * time.Millisecond //300--600
	DPrintf("节点%d刷新选举时间,超时时间是%d", rf.me, electionTimeout)
}
func (rf *Raft) becomeCandidate() {
	rf.state = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.ResetElectionTime()
}

func (rf *Raft) becomeLeader() {
	rf.state = Leader
	rf.resetTrackedIndex()

}
