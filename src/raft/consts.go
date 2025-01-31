package raft

import "time"

const (
	Follower  = 0
	Candidate = 1
	Leader    = 2
)

const (
	NoVote = -1 //未投票
)

const HeartTimeOut = 150 * time.Millisecond

const ElectionTimeoutBase = 300
