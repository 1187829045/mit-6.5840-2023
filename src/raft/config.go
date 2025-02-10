package raft

//
// support for Raft tester.
//
// we will use the original config.go to test your code for grading.
// so, while you can modify this code to help you debug, please
// test with the original before submitting.
//

import "6.5840/labgob"
import "6.5840/labrpc"
import "bytes"
import "log"
import "sync"
import "sync/atomic"
import "testing"
import "runtime"
import "math/rand"
import crand "crypto/rand"
import "math/big"
import "encoding/base64"
import "time"
import "fmt"

func randstring(n int) string {
	b := make([]byte, 2*n)
	crand.Read(b)
	s := base64.URLEncoding.EncodeToString(b)
	return s[0:n]
}

func makeSeed() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := crand.Int(crand.Reader, max)
	x := bigx.Int64()
	return x
}

type config struct {
	mu        sync.Mutex
	t         *testing.T
	finished  int32
	net       *labrpc.Network
	n         int
	rafts     []*Raft
	applyErr  []string // from apply channel readers
	connected []bool   // whether each server is on the net
	// saved 保存了每个服务器对应的 Persister 对象，用于存储持久化状态。
	saved       []*Persister
	endnames    [][]string            // the port file names each sends to
	logs        []map[int]interface{} // copy of each server's committed entries
	lastApplied []int
	start       time.Time // time at which make_config() was called
	// begin()/end() statistics
	t0        time.Time // time at which test_test.go called cfg.begin()
	rpcs0     int       // rpcTotal() at start of test
	cmds0     int       // number of agreements
	bytes0    int64
	maxIndex  int
	maxIndex0 int
}

var ncpu_once sync.Once

func make_config(t *testing.T, n int, unreliable bool, snapshot bool) *config {
	// 使用 `sync.Once` 保证 `ncpu_once` 的操作只执行一次。
	ncpu_once.Do(func() {
		// 如果 CPU 数小于 2，警告可能会隐藏锁定错误。
		if runtime.NumCPU() < 2 {
			fmt.Printf("warning: only one CPU, which may conceal locking bugs\n")
		}
		// 为随机数生成器设置种子。
		rand.Seed(makeSeed())
	})

	// 将 Go 的最大并行处理器数设置为 4，控制 Goroutine 的调度并发性。
	runtime.GOMAXPROCS(4)

	// 初始化配置结构体 `config`。
	cfg := &config{}
	cfg.t = t                                     // 测试框架 `t` 用于记录日志和错误。
	cfg.net = labrpc.MakeNetwork()                // 创建一个模拟网络，用于模拟节点之间的通信。
	cfg.n = n                                     // 配置节点数量。
	cfg.applyErr = make([]string, cfg.n)          // 每个节点的 apply 错误存储。
	cfg.rafts = make([]*Raft, cfg.n)              // 每个节点对应一个 Raft 实例。
	cfg.connected = make([]bool, cfg.n)           // 用于表示每个节点是否连接。
	cfg.saved = make([]*Persister, cfg.n)         // 保存每个节点的持久化状态。
	cfg.endnames = make([][]string, cfg.n)        // 存储每个节点的 RPC 端点名称。
	cfg.logs = make([]map[int]interface{}, cfg.n) // 存储每个节点的日志数据。
	cfg.lastApplied = make([]int, cfg.n)          // 跟踪每个节点的 `lastApplied` 索引。
	cfg.start = time.Now()                        // 记录配置初始化的开始时间。

	// 设置网络是否为不可靠模式。
	cfg.setunreliable(unreliable)

	// 配置网络延迟特性为长延迟模式。
	cfg.net.LongDelays(true)

	// 定义日志应用函数。如果启用了快照功能，使用快照处理方法。
	applier := cfg.applier
	if snapshot {
		applier = cfg.applierSnap
	}

	// 创建所有节点的 Raft 实例。
	for i := 0; i < cfg.n; i++ {
		// 初始化日志存储为空。
		cfg.logs[i] = map[int]interface{}{}
		// 启动每个节点并注册其日志应用方法。
		cfg.start1(i, applier)
	}

	// 将所有节点连接到网络中。
	for i := 0; i < cfg.n; i++ {
		cfg.connect(i)
	}

	// 返回初始化完成的配置。
	return cfg
}

// 关闭一个 Raft 服务器，但保留其持久化状态。
func (cfg *config) crash1(i int) {
	DPrintf(99, "this is crash1")
	cfg.disconnect(i)

	// 禁止客户端连接到服务器。
	cfg.net.DeleteServer(i)

	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	// 一个新的持久化存储对象，以防旧实例仍然在更新 Persister。
	// 但要复制旧 Persister 的内容，以确保我们始终将上次持久化的状态传递给 Make()。
	if cfg.saved[i] != nil {
		cfg.saved[i] = cfg.saved[i].Copy()
	}

	rf := cfg.rafts[i]
	if rf != nil {
		cfg.mu.Unlock()
		rf.Kill()
		cfg.mu.Lock()
		cfg.rafts[i] = nil
	}

	if cfg.saved[i] != nil {
		raftlog := cfg.saved[i].ReadRaftState()
		snapshot := cfg.saved[i].ReadSnapshot()
		cfg.saved[i] = &Persister{}
		cfg.saved[i].Save(raftlog, snapshot)
	}
}

// 作用是：检查每个节点的相同下标的日志的命令是否完全一致（如果不一致返回值错误信息，第一个返回值），并将i这个
//节点的 m.CommandIndex这个索引的命令设置为m.并更新最大索引，并返回m.CommandIndex是不是下标为0(第二个返回值)

func (cfg *config) checkLogs(i int, m ApplyMsg) (string, bool) {
	err_msg := ""
	v := m.Command
	for j := 0; j < len(cfg.logs); j++ {
		if old, oldok := cfg.logs[j][m.CommandIndex]; oldok && old != v {
			log.Printf("%v: log %v; server %v\n", i, cfg.logs[i], cfg.logs[j])
			// some server has already committed a different value for this entry!
			err_msg = fmt.Sprintf("commit index=%v server=%v %v != server=%v %v",
				m.CommandIndex, i, m.Command, j, old)
		}
	}
	_, prevok := cfg.logs[i][m.CommandIndex-1]
	cfg.logs[i][m.CommandIndex] = v
	if m.CommandIndex > cfg.maxIndex {
		cfg.maxIndex = m.CommandIndex
	}
	return err_msg, prevok
}

// applier reads message from apply ch and checks that they match the log
// contents
func (cfg *config) applier(i int, applyCh chan ApplyMsg) {
	for m := range applyCh {
		if m.CommandValid == false {
			// ignore other types of ApplyMsg
		} else {
			cfg.mu.Lock()
			err_msg, prevok := cfg.checkLogs(i, m)
			cfg.mu.Unlock()
			if m.CommandIndex > 1 && prevok == false {
				err_msg = fmt.Sprintf("server %v apply out of order %v", i, m.CommandIndex)
			}
			if err_msg != "" {
				log.Fatalf("apply error: %v", err_msg)
				cfg.applyErr[i] = err_msg
				// keep reading after error so that Raft doesn't block
				// holding locks...
			}
		}
	}
}

// 读取快照然后恢复状态机，这里的快照数据是持久化时产生的。
// ingestSnap 用于将从服务器 i 收到的快照数据加载到测试配置中，
// 并更新该服务器的已提交日志副本和 lastApplied 索引。
// returns "" or error string
func (cfg *config) ingestSnap(i int, snapshot []byte, index int) string {

	if snapshot == nil {
		// 如果传入的快照为 nil，则认为这是错误情况，终止程序运行。
		log.Fatalf("nil snapshot")
		return "nil snapshot" // 虽然 log.Fatalf 会退出，但为了语法完整性返回错误字符串。
	}

	// 使用快照字节数据创建一个缓冲区 r。
	r := bytes.NewBuffer(snapshot)

	// 创建 labgob 解码器 d，用于解码缓冲区中的快照数据。
	d := labgob.NewDecoder(r)

	// 定义变量 lastIncludedIndex，用于存放快照中包含的最后一个日志索引。
	var lastIncludedIndex int
	// 定义变量 xlog，用于存放快照中保存的日志条目（以 interface{} 切片存储）。
	var xlog []interface{}

	// 依次解码快照中的 lastIncludedIndex 和 xlog。
	// 如果解码过程中发生错误，则打印错误信息并退出程序。
	if d.Decode(&lastIncludedIndex) != nil ||
		d.Decode(&xlog) != nil {
		log.Fatalf("snapshot decode error")
		return "snapshot Decode() error"
	}

	// 如果传入的 index 参数不为 -1，并且它与快照中解码出的 lastIncludedIndex 不匹配，
	// 则说明该快照与预期的 SnapshotIndex 不一致，返回错误字符串。
	if index != -1 && index != lastIncludedIndex {
		err := fmt.Sprintf("server %v snapshot doesn't match m.SnapshotIndex", i)
		return err
	}

	// 将该服务器 i 的日志记录清空，创建一个新的空映射，存放快照中的日志条目。
	cfg.logs[i] = map[int]interface{}{}

	// 遍历解码出来的日志条目数组 xlog，
	// 将每个日志条目存入 cfg.logs[i] 中，索引从 0 开始递增。
	for j := 0; j < len(xlog); j++ {
		cfg.logs[i][j] = xlog[j]
	}

	// 更新该服务器 i 的 lastApplied 索引为快照中的最后包含日志的索引。
	cfg.lastApplied[i] = lastIncludedIndex

	// 返回空字符串，表示没有错误。
	return ""
}

const SnapShotInterval = 10 // 没九条日志生成一个快照

// periodically snapshot raft state
// 参数:一个是节点id，第二个是ApplyMsg的通道
//会从这个管道不断拿出数据来,然后分别对快照数据和日志数据分别处理
//如果是快照数据会加载到测试配置中,并更新该服务器的已提交日志副本和 lastApplied 索引。
//如果是命令也就是日志条目的话.会先检查命令下标处多个节点是否一样.并加入自己的日志数组中cfg.logs[i][m.CommandIndex] = v
//每九条日志生成一个快照,然后调用将快照发送给节点 rf.Snapshot.

func (cfg *config) applierSnap(i int, applyCh chan ApplyMsg) {
	cfg.mu.Lock()
	rf := cfg.rafts[i]
	cfg.mu.Unlock()
	if rf == nil {
		return
	}
	for m := range applyCh {
		err_msg := ""
		// 快照有效就读快照
		if m.SnapshotValid {
			cfg.mu.Lock()
			// ingestSnap 用于将从服务器 i 收到的快照数据(m.Snapshot)加载到测试配置中，
			// 并更新该服务器的已提交日志副本和 lastApplied 索引。
			err_msg = cfg.ingestSnap(i, m.Snapshot, m.SnapshotIndex)
			cfg.mu.Unlock()
		} else if m.CommandValid {
			// 命令有效就
			if m.CommandIndex != cfg.lastApplied[i]+1 {
				err_msg = fmt.Sprintf("server %v apply out of order, expected index %v, got %v", i, cfg.lastApplied[i]+1, m.CommandIndex)
			}

			if err_msg == "" {
				cfg.mu.Lock()
				var prevok bool
				// 作用是：检查每个节点的相同下标的日志的命令是否完全一致（
				//如果不一致返回值错误信息，第一个返回值），并将i这个
				//节点的 m.CommandIndex这个索引的命令设置为m.并更新最大索引，
				//并返回m.CommandIndex是不是下标为0(第二个返回值)
				err_msg, prevok = cfg.checkLogs(i, m)
				cfg.mu.Unlock()
				if m.CommandIndex > 1 && prevok == false {
					err_msg = fmt.Sprintf("server %v apply out of order %v", i, m.CommandIndex)
				}
			}

			cfg.mu.Lock()
			cfg.lastApplied[i] = m.CommandIndex
			cfg.mu.Unlock()

			if (m.CommandIndex+1)%SnapShotInterval == 0 {
				// 开始生成快照
				w := new(bytes.Buffer)
				e := labgob.NewEncoder(w)
				e.Encode(m.CommandIndex)
				var xlog []interface{}
				for j := 0; j <= m.CommandIndex; j++ {
					xlog = append(xlog, cfg.logs[i][j])
				}
				e.Encode(xlog)
				// 将快照发送给节点
				DPrintf(111, "开始生产快照")
				rf.Snapshot(m.CommandIndex, w.Bytes())
			}
		} else {
			// Ignore other types of ApplyMsg.
		}
		if err_msg != "" {
			log.Fatalf("apply error: %v", err_msg)
			cfg.applyErr[i] = err_msg
			// keep reading after error so that Raft doesn't block
			// holding locks...
		}
	}
}

// 启动或重启一个 Raft 实例。
// 如果已经存在，则先“杀死”它。
// 分配新的出站端口文件名和新的状态持久化器，以隔离该服务器的上一个实例。
// 因为我们实际上无法真正杀死它。
//并将快照信息放入测试的cfg.log中,并更新该服务器的已提交日志副本和 lastApplied 索引。
//然后启动一个协程,这个协程就是 cfg.applierSnap

func (cfg *config) start1(i int, applier func(int, chan ApplyMsg)) {
	// 如果节点已经存在，先模拟 "杀死" 该节点。
	cfg.crash1(i)

	// 为当前节点分配新的 ClientEnd 名称。
	// 这可以确保之前崩溃的实例的 ClientEnds 无法再发送消息。
	cfg.endnames[i] = make([]string, cfg.n)
	for j := 0; j < cfg.n; j++ {
		cfg.endnames[i][j] = randstring(20) // 为每个连接分配一个随机字符串。
	}

	// 创建一组新的 ClientEnd。
	ends := make([]*labrpc.ClientEnd, cfg.n)
	for j := 0; j < cfg.n; j++ {
		ends[j] = cfg.net.MakeEnd(cfg.endnames[i][j]) // 创建新的 ClientEnd。
		cfg.net.Connect(cfg.endnames[i][j], j)        // 将 ClientEnd 连接到指定的节点。
	}

	cfg.mu.Lock()

	// 重置该节点的 lastApplied 索引为 0。
	cfg.lastApplied[i] = 0

	// 分配一个新的持久化器 (Persister) 实例，以隔离旧实例的持久化状态。
	if cfg.saved[i] != nil { // 如果存在旧的持久化器。
		cfg.saved[i] = cfg.saved[i].Copy() // 创建旧持久化器的副本。

		// 检查是否有快照数据。
		snapshot := cfg.saved[i].ReadSnapshot()
		if snapshot != nil && len(snapshot) > 0 {
			// ingestSnap 用于将从服务器 i 收到的快照数据加载到测试配置中，
			// 并更新该服务器的已提交日志副本和 lastApplied 索引。
			err := cfg.ingestSnap(i, snapshot, -1)
			if err != "" {
				cfg.t.Fatal(err) // 如果处理快照出错，则触发测试失败。
			}
		}
	} else { // 如果没有旧持久化器，则创建一个新的。
		cfg.saved[i] = MakePersister()
	}

	cfg.mu.Unlock()

	// 创建一个用于接收 ApplyMsg 的通道。
	applyCh := make(chan ApplyMsg)

	// 使用新的配置，创建一个新的 Raft 实例。
	rf := Make(ends, i, cfg.saved[i], applyCh)

	cfg.mu.Lock()
	// 将新的 Raft 实例保存到配置中。
	cfg.rafts[i] = rf
	cfg.mu.Unlock()

	// 启动 applier 协程，处理 applyCh 中的消息。
	go applier(i, applyCh)

	// 创建一个 RPC 服务包装 Raft 实例。
	svc := labrpc.MakeService(rf)
	srv := labrpc.MakeServer()
	srv.AddService(svc)

	// 将 RPC 服务添加到网络中，供其他节点访问。
	cfg.net.AddServer(i, srv)
}

func (cfg *config) checkTimeout() {
	// enforce a two minute real-time limit on each test
	if !cfg.t.Failed() && time.Since(cfg.start) > 120*time.Second {
		cfg.t.Fatal("test took longer than 120 seconds")
	}
}

func (cfg *config) checkFinished() bool {
	z := atomic.LoadInt32(&cfg.finished)
	return z != 0
}

func (cfg *config) cleanup() {
	atomic.StoreInt32(&cfg.finished, 1)
	for i := 0; i < len(cfg.rafts); i++ {
		if cfg.rafts[i] != nil {
			cfg.rafts[i].Kill()
		}
	}
	cfg.net.Cleanup()
	cfg.checkTimeout()
}

// attach server i to the net.
func (cfg *config) connect(i int) {
	DPrintf(111, "connect(%d)\n", i)

	cfg.connected[i] = true

	// outgoing ClientEnds
	for j := 0; j < cfg.n; j++ {
		if cfg.connected[j] {
			endname := cfg.endnames[i][j]
			cfg.net.Enable(endname, true)
		}
	}

	// incoming ClientEnds
	for j := 0; j < cfg.n; j++ {
		if cfg.connected[j] {
			endname := cfg.endnames[j][i]
			cfg.net.Enable(endname, true)
		}
	}
}

// detach server i from the net.
func (cfg *config) disconnect(i int) {
	DPrintf(111, "disconnect(%d)\n", i)
	cfg.connected[i] = false

	// outgoing ClientEnds
	for j := 0; j < cfg.n; j++ {
		if cfg.endnames[i] != nil {
			endname := cfg.endnames[i][j]
			cfg.net.Enable(endname, false)
		}
	}

	// incoming ClientEnds
	for j := 0; j < cfg.n; j++ {
		if cfg.endnames[j] != nil {
			endname := cfg.endnames[j][i]
			cfg.net.Enable(endname, false)
		}
	}
}

func (cfg *config) rpcCount(server int) int {
	return cfg.net.GetCount(server)
}

func (cfg *config) rpcTotal() int {
	return cfg.net.GetTotalCount()
}

func (cfg *config) setunreliable(unrel bool) {
	cfg.net.Reliable(!unrel)
}

func (cfg *config) bytesTotal() int64 {
	return cfg.net.GetTotalBytes()
}

func (cfg *config) setlongreordering(longrel bool) {
	cfg.net.LongReordering(longrel)
}

func (cfg *config) checkOneLeader() int {
	// 进行最多 10 次迭代检查是否存在唯一的 leader。
	DPrintf(111, "进行最多 10 次迭代检查是否存在唯一的 leader。")
	for iters := 0; iters < 10; iters++ {
		DPrintf(111, "第%d次检查\n", iters+1)
		// 随机休眠 450-550 毫秒，模拟网络延迟或系统状态变化。
		ms := 450 + (rand.Int63() % 100)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		// 创建一个 map，用于记录每个任期 (term) 下的领导者节点 ID。
		leaders := make(map[int][]int)
		for i := 0; i < cfg.n; i++ { // 遍历所有节点。
			if cfg.connected[i] { // 检查节点是否已连接。
				// 获取节点的当前任期和是否是 leader 的状态。
				if term, leader := cfg.rafts[i].GetState(); leader {
					// 如果是 leader，将其 ID 添加到对应任期的领导者列表中。
					leaders[term] = append(leaders[term], i)
				}
			}
		}
		// 记录最近一个拥有 leader 的任期。
		lastTermWithLeader := -1
		for term, leaders := range leaders { // 遍历每个任期的领导者列表。
			if len(leaders) > 1 { // 如果一个任期中有多个领导者。
				// 立即触发测试失败，并输出错误信息。
				cfg.t.Fatalf("term %d has %d (>1) leaders", term, len(leaders))
			}
			if term > lastTermWithLeader { // 更新到最近的任期。
				lastTermWithLeader = term
			}
		}

		// 如果找到了至少一个任期的 leader，则返回最新任期的 leader 节点 ID。
		if len(leaders) != 0 {
			return leaders[lastTermWithLeader][0]
		}
	}

	// 如果所有迭代结束后仍未找到 leader，触发测试失败。
	cfg.t.Fatalf("expected one leader, got none")
	return -1 // 虽然不会执行到此处，但保留以满足函数返回值要求。
}

// check that everyone agrees on the term.
func (cfg *config) checkTerms() int {
	term := -1
	for i := 0; i < cfg.n; i++ {
		if cfg.connected[i] {
			xterm, _ := cfg.rafts[i].GetState()
			if term == -1 {
				term = xterm
			} else if term != xterm {
				cfg.t.Fatalf("servers disagree on term")
			}
		}
	}
	return term
}

// check that none of the connected servers
// thinks it is the leader.
func (cfg *config) checkNoLeader() {
	for i := 0; i < cfg.n; i++ {
		if cfg.connected[i] {
			_, is_leader := cfg.rafts[i].GetState()
			if is_leader {
				cfg.t.Fatalf("expected no leader among connected servers, but %v claims to be leader", i)
			}
		}
	}
}

// how many servers think a log entry is committed?
// 输入为提交项的索引，从1开始
// 作用：检查每个节点的index下标的日志是否相同，返回有多少个节点认为
// 第index数据已经提交，以及提交的日志项
func (cfg *config) nCommitted(index int) (int, interface{}) {
	DPrintf(111, "this is nCommitted")
	count := 0
	// 一个用来记录在index上各个实例存储的相同的日志项
	var cmd interface{} = nil
	// 遍历raft实例
	for i := 0; i < len(cfg.rafts); i++ {
		DPrintf(99, "检查节点%d下标%d日志是否一致", i, index)
		// cfg.applyErr数组负责存储 ”捕捉错误的协程“ 收集到的错误，
		//如果不空，则说明捕捉到异常
		if cfg.applyErr[i] != "" {
			cfg.t.Fatal(cfg.applyErr[i])
		}

		cfg.mu.Lock()
		// logs[i][index]负责存储检测线程提取到的每一个raft节点所有的提交项，
		//i是实例id，index是测试程序生成的日志项，
		// 如果某一个日志项在所有节点上的index位置上都被提交了，
		//则有logs[0][index]==logs[1][index]==logs[2][index]=...
		//==logs[n-1][index]
		cmd1, ok := cfg.logs[i][index]
		cfg.mu.Unlock()
		if ok {
			// 相反如果在index这个位置上有一个实例填充了数据（视为提交）但是不与其他实例相同，
			//则会发生不匹配的现象，抛异常
			// 如果某一个实例没有在这个位置上填充数据（等同没有提交），则cfg.logs[i][index]
			//的ok为false，此时虽然也不匹配但是不会抛异常
			DPrintf(111, "节点%d下标%d日志是 %T %v", i, index, cmd1, cmd1)
			if count > 0 && cmd != cmd1 {
				cfg.t.Fatalf("committed values do not match: index %v, %v, %v",
					index, cmd, cmd1)
			}
			count += 1
			cmd = cmd1
		} else {
			DPrintf(111, "cfg.logs[%d][%d]为空", i, index)
		}
	}
	return count, cmd // 返回有多少个节点认为第index数据已经提交，以及提交的日志项
}

// wait for at least n servers to commit.
// but don't wait forever.
func (cfg *config) wait(index int, n int, startTerm int) interface{} {
	to := 10 * time.Millisecond
	for iters := 0; iters < 30; iters++ {
		nd, _ := cfg.nCommitted(index)
		if nd >= n {
			break
		}
		time.Sleep(to)
		if to < time.Second {
			to *= 2
		}
		if startTerm > -1 {
			for _, r := range cfg.rafts {
				if t, _ := r.GetState(); t > startTerm {
					// someone has moved on
					// can no longer guarantee that we'll "win"
					return -1
				}
			}
		}
	}
	nd, cmd := cfg.nCommitted(index)
	if nd < n {
		cfg.t.Fatalf("only %d decided for index %d; wanted %d",
			nd, index, n)
	}
	return cmd
}

// do a complete agreement.
// it might choose the wrong leader initially,and have to re-submit after giving up.
// entirely gives up after about 10 seconds.
// indirectly checks that the servers agree on the same value, since nCommitted() checks this,
// as do the threads that read from applyCh.
// returns index.
// if retry==true, may submit the command multiple
// times, in case a leader fails just after Start().
// if retry==false, calls Start() only once, in order
// to simplify the early Lab 2B tests.

//完成整个协议。 它可能会在初始时选择错误的领导者，并且在放弃后需要重新提交。
//最终会在大约 10 秒后完全放弃。 间接检查服务器是否达成一致，
//因为 nCommitted() 会检查这一点，
//和从 applyCh 中读取的线程也会检查。 返回索引。 如果 retry==true，可能会多次提交命令，
//以防领导者在 Start() 之后失败。 如果 retry==false，仅调用一次 Start()，
//以简化早期的 Lab 2B 测试。

// 该方法的返回值是一个各个节点存储同一个日志项时放置的位置索引，它用于确定生产日志时
// 的顺序是否和
// 多数节点放置该日志的顺序一致。如果一致则说明存储位置正确，否则抛异常。
func (cfg *config) one(cmd interface{}, expectedServers int, retry bool) int {
	DPrintf(111, "this is one")
	t0 := time.Now()
	starts := 0
	// 10s内每隔50m循环检查，如果cfg节点没有挂掉（cfg.checkFinished==false）
	//就持续检测直到找到一个成为leader的节点，
	// 然后取到start函数生成的日志项在这个leader中日志数组的位置index，
	//然后再利用这个index去确认有多少从节点也复制并且提交了这个日志项
	count := 0 // 自己加的后面删
	for time.Since(t0).Seconds() < 10 && cfg.checkFinished() == false {
		// try all the servers, maybe one is the leader.
		count++                           // 自己加的后面删
		DPrintf(99, "One函数第%d次检查", count) // 自己加的后面删
		index := -1
		for si := 0; si < cfg.n; si++ {
			starts = (starts + 1) % cfg.n
			var rf *Raft
			cfg.mu.Lock()
			if cfg.connected[starts] {
				rf = cfg.rafts[starts]
			}
			cfg.mu.Unlock()
			if rf != nil {
				//rf的start函数，会返回该日志项在leader节点中的日志数组的
				//索引位置，任期以及是否是leader，如果找到了leader则break
				index1, _, ok := rf.Start(cmd)
				if ok {
					index = index1
					DPrintf(111, "找到Leader节点是%d，任期是%d,LastLogIndex下标是%d,break ", rf.me, rf.currentTerm, index1)
					break
				}
			}
		}
		// index不等于-1则表示找到了一个存储了cmd日志项的leader节点
		if index != -1 {
			// somebody claimed to be the leader and to have
			// submitted our command; wait a while for agreement.
			t1 := time.Now()
			// 下面这个循环的意思是每隔20ms就轮询一次已经提交内容为cmd的日志项的节点数量是否大于等于expectedServers
			// 为什么是2s内呢？因为正常情况下2s内一定能确认所有的节点都能够提交成功
			count1 := 0 //自己加的
			DPrintf(111, "开始循环检查检查下标%d日志是否一致", index)
			for time.Since(t1).Seconds() < 2 {
				count1++
				DPrintf(99, "第%d次检查下标%d日志是否一致", count1, index)
				nd, cmd1 := cfg.nCommitted(index)
				// 如果是则比较在这个索引位置上各节点提交的日志是否和给定的日志相同，如果相同直接返回索引
				if nd > 0 && nd >= expectedServers {
					// committed
					if cmd1 == cmd {
						// 返回index是为了确认各个节点提交的索引位置是否和start生成的顺序是否一致
						// and it was the command we submitted.
						return index
					}
				}
				time.Sleep(20 * time.Millisecond)
			}
			// 如果不是则看是否重试，不允许重试就抛异常
			if retry == false {
				cfg.t.Fatalf("one(%v) failed to reach agreement", cmd)
			}
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if cfg.checkFinished() == false {
		// 如果在
		cfg.t.Fatalf("one(%v) failed to reach agreement", cmd)
	}
	return -1
}

// start a Test.
// print the Test message.
// e.g. cfg.begin("Test (2B): RPC counts aren't too high")
func (cfg *config) begin(description string) {
	fmt.Printf("%s ...\n", description)
	cfg.t0 = time.Now()
	cfg.rpcs0 = cfg.rpcTotal()
	cfg.bytes0 = cfg.bytesTotal()
	cfg.cmds0 = 0
	cfg.maxIndex0 = cfg.maxIndex
}

// end a Test -- the fact that we got here means there
// was no failure.
// print the Passed message,
// and some performance numbers.
func (cfg *config) end() {
	cfg.checkTimeout()
	if cfg.t.Failed() == false {
		cfg.mu.Lock()
		t := time.Since(cfg.t0).Seconds()       // real time
		npeers := cfg.n                         // number of Raft peers
		nrpc := cfg.rpcTotal() - cfg.rpcs0      // number of RPC sends
		nbytes := cfg.bytesTotal() - cfg.bytes0 // number of bytes
		ncmds := cfg.maxIndex - cfg.maxIndex0   // number of Raft agreements reported
		cfg.mu.Unlock()

		fmt.Printf("  ... Passed --")
		fmt.Printf("  %4.1f  %d %4d %7d %4d\n", t, npeers, nrpc, nbytes, ncmds)
	}
}

// Maximum log size across all servers
func (cfg *config) LogSize() int {
	logsize := 0
	for i := 0; i < cfg.n; i++ {
		n := cfg.saved[i].RaftStateSize()
		if n > logsize {
			logsize = n
		}
	}
	return logsize
}
