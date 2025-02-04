package raft

import (
	"6.5840/labrpc"
	"sync"
	"time"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	state            int           // 节点状态，Candidate-Follower-Leader
	currentTerm      int           // 当前的任期
	votedFor         int           // 投票给谁
	heartbeatTimeout time.Duration // 心跳定时器
	electionTimeout  time.Duration //选举计时器
	lastElection     time.Time     // 上一次的选举时间，用于配合since方法计算当前的选举时间是否超时
	lastHeartbeat    time.Time     // 上一次的心跳时间，用于配合since方法计算当前的心跳时间是否超时
	peerTrackers     []PeerTracker // Leader专属：keeps track of each peer's next index, match index, etc.
	log              *Log          // 日志记录

	//Volatile state
	commitIndex int // commitIndex是本机提交的
	lastApplied int // lastApplied是该日志在所有的机器上都跑了一遍后才会更新？

	applyHelper *ApplyHelper
	applyCond   *sync.Cond
}

type RequestAppendEntriesArgs struct {
	LeaderTerm   int // Leader的Term
	LeaderId     int
	PrevLogIndex int // 新日志条目的上一个日志的索引
	PrevLogTerm  int // 新日志的上一个日志的任期
	//Logs         []ApplyMsg // 需要被保存的日志条目,可能有多个
	Entries      []Entry
	LeaderCommit int // Leader已提交的最高的日志项目的索引
}

type RequestAppendEntriesReply struct {
	FollowerTerm int  // Follower的Term,给Leader更新自己的Term
	Success      bool // 是否推送成功
	PrevLogIndex int
	PrevLogTerm  int
}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term        int // candidate’s term
	CandidateId int //candidate requesting vote

	LastLogIndex int // index of candidate’s last log entry (§5.4)
	LastLogTerm  int //term of candidate’s last log entry
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}
type Entry struct {
	Term    int
	Command interface{}
}
type Log struct {
	Entries       []Entry
	FirstLogIndex int
	LastLogIndex  int
}
