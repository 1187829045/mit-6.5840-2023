package raft

import (
	"log"
	"sync"
	"sync/atomic"
)

// Debugging
const Debug = true

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}
func max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}
func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}
func ifCond(cond bool, a, b interface{}) interface{} {
	if cond {
		return a
	} else {
		return b
	}
}
func getcount() func() int {
	cnt := 1000
	return func() int {
		cnt++
		return cnt
	}
}

type ApplyHelper struct {
	applyCh       chan ApplyMsg
	lastItemIndex int
	q             []ApplyMsg
	mu            sync.Mutex
	cond          *sync.Cond
	dead          int32
}

func NewApplyHelper(applyCh chan ApplyMsg, lastApplied int) *ApplyHelper {
	applyHelper := &ApplyHelper{
		applyCh:       applyCh,
		lastItemIndex: lastApplied,
		q:             make([]ApplyMsg, 0),
	}
	applyHelper.cond = sync.NewCond(&applyHelper.mu)
	go applyHelper.applier()
	return applyHelper
}
func (applyHelper *ApplyHelper) Kill() {
	atomic.StoreInt32(&applyHelper.dead, 1)
}
func (applyHelper *ApplyHelper) killed() bool {
	z := atomic.LoadInt32(&applyHelper.dead)
	return z == 1
}
func (applyHelper *ApplyHelper) applier() {
	for !applyHelper.killed() {
		applyHelper.mu.Lock()
		if len(applyHelper.q) == 0 {
			applyHelper.cond.Wait()
		}
		msg := applyHelper.q[0]
		applyHelper.q = applyHelper.q[1:]
		applyHelper.mu.Unlock()
		DPrintf("applyhelper start apply msg index=%v ", ifCond(msg.CommandValid, msg.CommandIndex, msg.SnapshotIndex))
		applyHelper.applyCh <- msg
		DPrintf("applyhelper done apply msg index=%v", ifCond(msg.CommandValid, msg.CommandIndex, msg.SnapshotIndex))
	}
}
func (applyHelper *ApplyHelper) tryApply(msg *ApplyMsg) bool {
	applyHelper.mu.Lock()
	defer applyHelper.mu.Unlock()
	DPrintf("applyhelper 获得消息,下标是=%v", ifCond(msg.CommandValid, msg.CommandIndex, msg.SnapshotIndex))
	if msg.CommandValid {
		if msg.CommandIndex <= applyHelper.lastItemIndex {
			return true
		}
		if msg.CommandIndex == applyHelper.lastItemIndex+1 {
			applyHelper.q = append(applyHelper.q, *msg)
			applyHelper.lastItemIndex++
			applyHelper.cond.Broadcast()
			return true
		}
		panic("applyhelper meet false")
		return false
	} else if msg.SnapshotValid {
		if msg.SnapshotIndex <= applyHelper.lastItemIndex {
			return true
		}
		applyHelper.q = append(applyHelper.q, *msg)
		applyHelper.lastItemIndex = msg.SnapshotIndex
		applyHelper.cond.Broadcast()
		return true
	} else {
		panic("applyHelper meet both invalid")
	}
}

// 通知tester接收这个日志消息，然后供测试使用
func (rf *Raft) sendMsgToTester() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	for !rf.killed() {
		rf.applyCond.Wait()
		for rf.lastApplied < rf.commitIndex {
			i := rf.lastApplied + 1
			rf.lastApplied++
			msg := ApplyMsg{
				CommandValid: true,
				Command:      rf.log.getOneEntry(i).Command,
				CommandIndex: i,
			}
			rf.applyHelper.tryApply(&msg)
		}
	}
}
