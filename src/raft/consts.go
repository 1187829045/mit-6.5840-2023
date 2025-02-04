package raft

import "time"

const None = -1

// 设置状态类型
const Follower, Candidate, Leader int = 1, 2, 3
const tickInterval = 70 * time.Millisecond
const heartbeatTimeout = 150 * time.Millisecond
const baseElectionTimeout = 300
