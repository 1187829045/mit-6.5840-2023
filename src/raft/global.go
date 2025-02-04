package raft

import (
	"math/rand"
	"time"
)

func (rf *Raft) pastHeartbeatTimeout() bool {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return time.Since(rf.lastHeartbeat) > rf.heartbeatTimeout
}

func (rf *Raft) resetHeartbeatTimer() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.lastHeartbeat = time.Now()
}
func (rf *Raft) getLastEntryTerm() int {
	if rf.log.LastLogIndex >= rf.log.FirstLogIndex {
		return rf.log.getOneEntry(rf.log.LastLogIndex).Term
	}
	return -1

}
func (rf *Raft) getEntryTerm(index int) int {
	if index == 0 {
		return 0
	}
	if rf.log.FirstLogIndex <= rf.log.LastLogIndex {
		return rf.log.getOneEntry(index).Term
	}

	DPrintf(999, "invalid index=%v in getEntryTerm rf.log.FirstLogIndex=%v rf.log.LastLogIndex=%v\n", index, rf.log.FirstLogIndex, rf.log.LastLogIndex)
	return -1
}
func (rf *Raft) pastElectionTimeout() bool {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return time.Since(rf.lastElection) > rf.electionTimeout
}

func (rf *Raft) resetElectionTimer() {
	electionTimeout := baseElectionTimeout + (rand.Int63() % baseElectionTimeout)
	rf.electionTimeout = time.Duration(electionTimeout) * time.Millisecond
	rf.lastElection = time.Now()
}

func (rf *Raft) becomeCandidate() {
	rf.state = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.resetElectionTimer()
}

func (rf *Raft) becomeLeader() {
	rf.state = Leader
	rf.resetTrackedIndex()

}
