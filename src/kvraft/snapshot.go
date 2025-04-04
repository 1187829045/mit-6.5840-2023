package kvraft

import (
	"6.5840/labgob"
	"bytes"
)

func (kv *KVServer) isNeedSnapshot() bool {
	if kv.maxraftstate == -1 {
		return false
	}
	_len := kv.persister.RaftStateSize()
	return _len-100 >= kv.maxraftstate
}

// 制作快照
func (kv *KVServer) makeSnapshot(index int) {
	_, isleader := kv.rf.GetState()
	if !isleader {
		return
	}
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	kv.mu.Lock()
	defer kv.mu.Unlock()

	e.Encode(kv.kvPersist)
	e.Encode(kv.seqMap)
	snapshot := w.Bytes()
	// 快照完马上递送给leader节点
	go kv.rf.Snapshot(index, snapshot)
}

// 解码快照
func (kv *KVServer) decodeSnapshot(index int, snapshot []byte) {

	// 这里必须判空，因为当节点第一次启动时，持久化数据为空，如果还强制读取数据会报错
	if snapshot == nil || len(snapshot) < 1 {
		DPrintf(11111, "持久化数据为空！")
		return
	}

	r := bytes.NewBuffer(snapshot)
	d := labgob.NewDecoder(r)

	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.lastIncludeIndex = index
	DPrintf(11111, "应用快照前，数据库中的数据为%v, 请求状态为%v", kv.kvPersist, kv.seqMap)

	if d.Decode(&kv.kvPersist) != nil || d.Decode(&kv.seqMap) != nil {
		panic("error in parsing snapshot")
	}
}

// 将快照传递给leader节点，再由leader节点传递给各个从节点
//func (kv *KVServer) deliverSnapshot() {
//	_, isleader := kv.rf.GetState()
//	if !isleader {
//		return
//	}
//	kv.mu.Lock()
//	defer kv.mu.Unlock()
//	DPrintf(1111, "准备发送")
//	r := bytes.NewBuffer(kv.snapshot)
//	d := labgob.NewDecoder(r)
//	var lastApplied int
//	if d.Decode(&lastApplied) != nil {
//		DPrintf(999, "%d: state machine reads Snapshot decode error\n", kv.me)
//		panic(" state machine reads Snapshot decode error")
//	}
//	kv.rf.Snapshot(lastApplied, kv.snapshot)
//}
