package raft

import (
	"6.5840/labrpc"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// 一个实现单个 Raft 节点的 Go 对象
type Config struct {
	mu          sync.Mutex
	t           *testing.T            // 测试对象，用于在测试失败时输出错误信息
	finished    int32                 // 标记测试是否完成（可能用于并发控制）
	net         *labrpc.Network       // 网络对象，管理 Raft 节点之间的通信
	n           int                   // Raft 集群中的节点数量
	rafts       []*Raft               // Raft 节点的切片，保存每个节点的 Raft 实例
	applyErr    []string              // 存储应用通道的错误信息
	connected   []bool                // 标记每个节点是否与网络连接
	saved       []*Persister          // 每个节点的持久化存储，保存 Raft 状态
	endnames    [][]string            // 每个节点发送到其他节点的端口文件名
	logs        []map[int]interface{} // 每个节点已提交日志条目的副本
	lastApplied []int                 // 每个节点最后应用的日志条目的索引
	start       time.Time             // 调用 make_config() 时的时间，标记配置开始的时间
	// 以下为 begin()/end() 统计信息
	t0        time.Time // 测试开始时的时间（在 test_test.go 中调用 cfg.begin()）
	rpcs0     int       // 测试开始时的总 RPC 调用次数
	cmds0     int       // 测试开始时达成一致的命令数量
	bytes0    int64     // 测试开始时传输的字节数
	maxIndex  int       // 当前最大日志索引
	maxIndex0 int       // 测试开始时的最大日志索引
}

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

	applyHelper *ApplyHelper
	applyCond   *sync.Cond
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
	Term         int //领导者的 任期
	LeaderId     int // 领导者的 ID。
	PrevLogIndex int // 紧邻新日志条目的索引
	PrevLogTerm  int // 紧邻新日志条目的term

	Entries      []Entry //需要存储的日志条目（心跳时为空）。
	LeaderCommit int     //领导者的 commitIndex。
}

type AppendEntriesReply struct {
	Term         int
	Success      bool
	PrevLogIndex int
	PrevLogTerm  int
}

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
	//DPrintf("节点%d刷新选举时间,超时时间是%d", rf.me, electionTimeout)
}
func (rf *Raft) becomeCandidate() {
	rf.state = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.ResetElectionTime()
}

func (rf *Raft) becomeLeader() {
	DPrintf("节点%d成为Leader", rf.me)
	rf.state = Leader
	rf.resetTrackedIndex()

}
