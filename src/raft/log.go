package raft

import (
	"fmt"
)

func NewLog() *Log {
	return &Log{
		Entries:       make([]Entry, 0),
		FirstLogIndex: 1, //测试是从1开始检查和添加的
		LastLogIndex:  0,
	}
}
func (log *Log) getRealIndex(index int) int {
	return index - log.FirstLogIndex
}
func (log *Log) getOneEntry(index int) *Entry {
	return &log.Entries[log.getRealIndex(index)]
}

func (log *Log) appendL(newEntries ...Entry) {
	log.Entries = append(log.Entries[:log.getRealIndex(log.LastLogIndex)+1], newEntries...)
	log.LastLogIndex += len(newEntries)

}
func (log *Log) getAppendEntries(start int) []Entry {
	ret := append([]Entry{}, log.Entries[log.getRealIndex(start):log.getRealIndex(log.LastLogIndex)+1]...)
	return ret
}
func (log *Log) String() string {
	if log.empty() {
		return "logempty"
	}
	return fmt.Sprintf("%v", log.getAppendEntries(log.FirstLogIndex))
}
func (log *Log) empty() bool {
	return log.FirstLogIndex > log.LastLogIndex
}
func (rf *Raft) GetLogEntries() []Entry {
	return rf.log.Entries
}
