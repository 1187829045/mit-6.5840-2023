package raft

type Entry struct {
	Term    int
	Command interface{}
}

type Log struct {
	Entries      []Entry
	LastLogIndex int
}

func NewLog() *Log {
	entries := make([]Entry, 0)
	entries = append(entries, Entry{Term: -1, Command: nil})
	return &Log{
		Entries:      entries,
		LastLogIndex: 0,
	}
}
func (log *Log) getOneEntry(index int) *Entry {
	return &log.Entries[index]
}

func (log *Log) appendEntry(newEntries ...Entry) {
	log.Entries = append(log.Entries, newEntries...)
	log.LastLogIndex += len(newEntries)
}
func (log *Log) getAppendEntries(start int) []Entry { //前闭后开
	ret := append([]Entry{}, log.Entries[start:]...)
	return ret
}
func (rf *Raft) getEntryTerm(index int) int {
	if index < 0 {
		return -1
	}
	return rf.log.getOneEntry(index).Term
}
func (rf *Raft) getLastEntryTerm() int {
	if rf.log.LastLogIndex == -1 {
		return -1
	}
	return rf.log.getOneEntry(rf.log.LastLogIndex).Term
}
