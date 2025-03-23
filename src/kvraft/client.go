package kvraft

import (
	"6.5840/labrpc"
	"crypto/rand"
	"math/big"
	"sync"
	"time"
)

const interval = 100 * time.Millisecond

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	mu sync.Mutex

	clientId   int // 客户端id
	ProposalId int // 当前客户端发出的第几个请求

}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.

	ck.clientId = int(nrand()) % len(ck.servers)
	//ck.clientId = int(nrand())
	ck.ProposalId = 0 //初始化为0，第一个请求从1开始
	DPrintf(1111, "create a client with id %d and proposal 0", ck.clientId)
	return ck
}

// Get 获取指定键的当前值。 如果键不存在，则返回空字符串""。
// 在面对其他所有错误时，会无限重试直到成功。
// 你可以通过如下方式发送 RPC 调用：
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
// 注意：参数 args 和 reply 的类型（包括是否为指针类型）
// 必须与 RPC 处理函数声明的参数类型严格匹配。且 reply 必须作为指针类型传递。
func (ck *Clerk) Get(key string) string {
	ck.mu.Lock()
	ck.ProposalId++
	// 统一的请求参数
	args := GetArgs{}
	args.Key = key
	args.ClientId = ck.clientId
	args.ProposalId = ck.ProposalId
	ck.mu.Unlock()

	DPrintf(1111, "发送get参数: ClientId: %d, ProposalId: %d, key: %v",
		args.ClientId, args.ProposalId, args.Key)

	res := ""
	for true {
		for i := 0; i < len(ck.servers); i++ {
			reply := GetReply{}
			ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
			if !ok {
				DPrintf(1111, "no rpc response for Get request")
				continue
			}
			if reply.Err == OK {
				res = reply.Value
				DPrintf(1111, "the query result is %v!!", res)
				return res
			} else {
				DPrintf(1111, "err is %v, and retry!", reply.Err)
			}

		}
		time.Sleep(interval)
	}
	return res
}

// PutAppend  Put 和 Append 操作共用该方法。
// 你可以通过如下方式发送 RPC 调用：
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
// 注意：参数 args 和 reply 的类型（包括是否为指针类型） 必须与 RPC 处理函数声明的参数类型严格匹配。且 reply 必须作为指针类型传递。
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	ck.mu.Lock()
	ck.ProposalId++
	args := PutAppendArgs{}
	args.ClientId = ck.clientId
	args.ProposalId = ck.ProposalId
	ck.mu.Unlock()
	args.Key = key
	args.Value = value
	args.Op = op
	DPrintf(1111, "put: ClientId: %d, ProposalId: %d, args.key: %v, and args.value: %v",
		args.ClientId, args.ProposalId, args.Key, args.Value)

	for true {
		for i := 0; i < len(ck.servers); i++ {
			reply := PutAppendReply{}
			DPrintf(1111, "准备尝试调用节点%d的PutAppend方法", i)

			ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
			if !ok {
				DPrintf(1111, "no rpc response for Put request")
				continue
			}
			//DPrintf(1111, "已经调用节点%d的PutAppend方法", i)
			if reply.Err == OK {
				//res = reply.Value
				DPrintf(1111, "成功执行put操作， ClientId: %d, ProposalId: %d, args.key: %v, and args.value: %v",
					args.ClientId, args.ProposalId, args.Key, args.Value)

				return
			}

		}
		time.Sleep(interval)
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
