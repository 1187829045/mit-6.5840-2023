package raft

import (
	"6.5840/labrpc"
	"sync"
	"time"
)

type PeerTracker struct {
	nextIndex  int
	matchIndex int

	lastAck time.Time
}

// 将 Raft 的持久化状态保存到稳定存储，
// 在崩溃和重启后可以重新加载。
// 参考论文 Figure 2 描述的持久化状态。

type Raft struct {
	mu        sync.Mutex          // 锁，用于保护共享状态
	peers     []*labrpc.ClientEnd // 所有节点的 RPC 端点
	persister *Persister          // 用于保存节点持久化状态
	me        int                 // 当前节点在 peers[] 数组中的索引
	dead      int32               // 节点是否被 Kill() 设置为死亡状态

	// 你的数据字段（用于 2A, 2B, 2C）。
	// 参考论文 Figure 2 描述的 Raft 节点状态。
	commitIndex      int
	lastApplied      int
	state            int           // 节点状态，Candidate-Follower-Leader
	currentTerm      int           // 当前的任期
	votedFor         int           // 投票给谁
	heartbeatTimeout time.Duration // 心跳定时器
	electionTimeout  time.Duration // 选举计时器
	lastElection     time.Time     // 上一次的选举时间，用于配合since方法计算当前的选举时间是否超时
	lastHeartbeat    time.Time     // 上一次的心跳时间，用于配合since方法计算当前的心跳时间是否超时
	peerTrackers     []PeerTracker //从节点的状态
	log              *Log
}

// 每当一个新的日志条目被提交时，Raft 节点需要通过 applyCh 发送 ApplyMsg。
// 设置 CommandValid 为 true 表示 ApplyMsg 包含一个新的提交日志条目。
//
// 在 2D 部分你需要通过 applyCh 发送其他类型的消息（例如快照）。
// 对于这些消息，设置 CommandValid 为 false。

type ApplyMsg struct {
	CommandValid bool        // 是否是有效的命令
	Command      interface{} // 具体的命令
	CommandIndex int         // 命令的索引

	// 适用于 2D：
	SnapshotValid bool   // 是否是有效的快照
	Snapshot      []byte // 快照数据
	SnapshotTerm  int    // 快照对应的任期号
	SnapshotIndex int    // 快照的最后索引
}

// RequestVote RPC 的请求参数结构。
// 字段名必须以大写字母开头！

type RequestVoteArgs struct {
	// 你的数据（用于 2A, 2B）。
	//term: 候选人的任期号。
	//candidateId: 候选人 ID。
	//lastLogIndex: 候选人最后日志条目的索引。
	//lastLogTerm: 候选人最后日志条目的任期号。

	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// RequestVote RPC 的回复参数结构。
// 字段名必须以大写字母开头！

type RequestVoteReply struct {
	// 你的数据（用于 2A）。
	//term: 接收者的当前任期号（供候选人更新）。
	//voteGranted: 是否投票支持候选人。

	Term        int
	VoteGranted bool
}
type AppendEntriesArgs struct {
	Term         int     //领导者的 任期
	LeaderId     int     // 领导者的 ID。
	PrevLogIndex int     // 紧邻新日志条目的索引
	PrevLogTerm  int     // 紧邻新日志条目的term
	Entries      []Entry //需要存储的日志条目（心跳时为空）。
	LeaderCommit int     //领导者的 commitIndex。
}

type AppendEntriesReply struct {
	Term         int
	Success      bool
	PrevLogIndex int
	PrevLogTerm  int
}
