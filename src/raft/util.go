package raft

import (
	"log"
	"sync"
	"sync/atomic"
)

// Debugging
const Debug_level = 100

func DPrintf(level int, format string, a ...interface{}) {
	if Debug_level <= level {
		log.Printf(format, a...)
	}
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
		DPrintf(99, "applier 开始从自身队列中取数据放入applyCh管道中")
		msg := applyHelper.q[0]
		DPrintf(99, "msg=%v", msg)
		applyHelper.q = applyHelper.q[1:]
		applyHelper.mu.Unlock()
		applyHelper.applyCh <- msg
	}
}
func (applyHelper *ApplyHelper) tryApply(msg *ApplyMsg) bool {
	applyHelper.mu.Lock()
	defer applyHelper.mu.Unlock()
	DPrintf(99, "tryApply向自身数组放入数据")
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
		DPrintf(99, "msg.CommandIndex =%d,applyHelper.lastItemIndex=%d",
			msg.CommandIndex, applyHelper.lastItemIndex)
		panic("tryApply 出错")
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
