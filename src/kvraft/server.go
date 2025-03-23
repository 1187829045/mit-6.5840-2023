package kvraft

import (
	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raft"
	"fmt"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
)

// Debug_level Debugging
const Debug_level = 1000

func DPrintf(level int, format string, a ...interface{}) (n int, err error) {
	if Debug_level <= level {
		log.Printf(format, a...)
	}
	return
}

const GetOp, AppendOp, PutOp string = "Get", "Append", "Put"

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	OpType     string // 操作类型
	Key        string // 待操作的键
	Val        string // 待操作的值
	ClientId   int
	ProposalId int
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.

	KvDB              map[string]string // 数据库
	OpMap             map[string]string // 操作去重用的map
	handleRequestCond *sync.Cond        // 同步工具，当命令在状态机中执行完毕后就可以通知执行get，append以及put的线程了
}

func (kv *KVServer) printCommand(op Op) string {
	str := fmt.Sprintf("key: %s, val: %s, op: %s, clientId: %d, proposalId: %d", op.Key, op.Val, op.OpType, op.ClientId, op.ProposalId)
	return str
}

// 用于执行applyCh中的命令
func (kv *KVServer) executeCommand() {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	for !kv.killed() {
		op := <-kv.applyCh
		if op.CommandValid {
			cmd, ok := op.Command.(Op)
			if ok {
				// 如果是查询操作
				if cmd.OpType == GetOp {
					DPrintf(1111, "已确保读操作同步到各个raft节点，保证了读一致性")

				} else if cmd.OpType == PutOp {
					// 插入操作
					kv.KvDB[cmd.Key] = cmd.Val
					DPrintf(1111, "Put操作成功执行")
					//DPrintf(1111, "准备通知leader节点")

				} else {
					// 追加操作
					kv.KvDB[cmd.Key] += cmd.Val
					DPrintf(1111, "Append操作成功执行")
				}
				//str := "key:" + cmd.Key + ", val: " + cmd.Val + ", op:" + cmd.OpType + ", clientId:" + cmd.ClientId + ", prosopalId:" + cmd.ProposalId
				DPrintf(1111, "执行命令{%v}成功", kv.printCommand(cmd))
				kv.OpMap[strconv.Itoa(cmd.ClientId)+":"+strconv.Itoa(cmd.ProposalId)] = "Exist"
				_, isLeader := kv.rf.GetState()
				DPrintf(1111, "准备通知leader节点")
				if isLeader {
					kv.handleRequestCond.Broadcast()
					DPrintf(1111, "成功通知leader节点")
				}
			}
		}
	}
}
func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.

	DPrintf(1111, "接收get参数: ClientId: %d, ProposalId: %d, key: %v",
		args.ClientId, args.ProposalId, args.Key)
	// 过滤掉重复请求
	_, exist := kv.OpMap[strconv.Itoa(args.ClientId)+":"+strconv.Itoa(args.ProposalId)]
	if exist {
		DPrintf(1111, "重复的请求，直接返回成功")

		reply.Err = OK
		reply.Value = kv.KvDB[args.Key]
		return
	}

	op := Op{}
	op.OpType = GetOp
	op.Key = args.Key
	op.Val = ""
	op.ClientId = args.ClientId
	op.ProposalId = args.ProposalId
	// 将整个命令传递给raft leader
	// raft不执行命令，只负责将命令副本分发给各个从节点
	_, _, isLeader := kv.rf.Start(op)

	if !isLeader {
		reply.Err = ErrWrongLeader
		return
	}
	// 如果是leader就需要等待leader将数据同步完成
	kv.handleRequestCond.Wait()

	// 状态机应用此查询完毕，开始查询
	selectVal, _ := kv.KvDB[args.Key]
	reply.Value = selectVal
	reply.Err = OK

}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	DPrintf(1111, "put: 收到clientClientId: %d的调用请求，其序列号为 ProposalId: %d, 其他参数：args.key: %v, and args.value: %v",
		args.ClientId, args.ProposalId, args.Key, args.Value)
	// 过滤掉重复请求
	_, exist := kv.OpMap[strconv.Itoa(args.ClientId)+":"+strconv.Itoa(args.ProposalId)]
	if exist {
		DPrintf(1111, "重复的请求，直接返回成功")
		reply.Err = OK
		return
	}
	DPrintf(1111, "准备将命令发送给状态机")

	op := Op{}
	op.OpType = args.Op
	op.Key = args.Key
	op.Val = args.Value
	op.ClientId = args.ClientId
	op.ProposalId = args.ProposalId
	// 将整个命令传递给raft leader
	// raft不执行命令，只负责将命令副本分发给各个从节点
	_, _, isLeader := kv.rf.Start(op)

	if !isLeader {
		DPrintf(1111, "不是leader，直接返回")
		reply.Err = ErrWrongLeader
		return
	}
	DPrintf(1111, "是leader, 但是要等到日志同步完成...")
	// 如果是leader就需要等待leader将数据同步完成
	kv.handleRequestCond.Wait()

	// 开始执行命令
	if args.Op == PutOp {
		kv.KvDB[args.Key] = args.Value
	} else if args.Op == AppendOp {
		kv.KvDB[args.Key] += args.Value
	}
	reply.Err = OK
	DPrintf(1111, "PutAppend成功执行！！！")

}

// Kill 当测试程序判定某个 KVServer 实例不再需要时，会调用 Kill() 方法。
// 我们已提供以下便利功能：
// 1. 设置 rf.dead 的线程安全代码（无需额外加锁）
// 2. killed() 方法用于在长时间运行的循环中检测实例状态
// 开发者可自由扩展 Kill() 方法（非强制要求），
// 典型应用场景：停止被终止实例的调试日志输出
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// StartKVServer servers[] 存储集群中所有服务器的端口地址，
// 这些服务器将通过 Raft 共识算法协作构建高可用的键值存储服务。
// me 表示当前服务器在 servers[] 数组中的索引位置。
// 键值服务必须通过底层 Raft 实现快照存储功能，
// 具体应调用 persister.SaveStateAndSnapshot() 方法
// 以原子操作同时保存 Raft 状态与快照数据。
// 当 Raft 持久化状态超过 maxraftstate 字节阈值时，
// 键值服务需主动生成快照，以便 Raft 进行日志垃圾回收。
// 若 maxraftstate 为 -1 则表示禁用快照功能。
// StartKVServer() 必须快速返回，因此应当为
// 所有耗时操作启动独立的 goroutine。

func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.
	kv.dead = 0
	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)
	kv.OpMap = make(map[string]string)
	kv.KvDB = make(map[string]string)
	kv.handleRequestCond = sync.NewCond(&kv.mu)

	// You may need initialization code here.
	go kv.executeCommand()
	return kv
}
