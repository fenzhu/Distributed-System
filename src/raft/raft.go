package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import "sync"
import "sync/atomic"
import "../labrpc"
import "time"
import "math/rand"
// import "bytes"
// import "../labgob"


const NotVoted int = -100
//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type LogEntry struct {
	Command interface{}
	Term int
}
//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	ch chan ApplyMsg
	//persistent
	currentTerm int
	votedFor int
	log []LogEntry
	
	//volatile
	CommitIndex int
	LastApplied int

	//for live check
	leaderIndex int
	heartbeatTime time.Time

	mx sync.Mutex
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool

	term = rf.currentTerm
	isleader = rf.me == rf.leaderIndex
	// Your code here (2A).
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}


//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}




//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term int
	CandidateId int
	LastLogIndex int
	LastLogTerm int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
		Term int
		VoteGranted bool
}

type AppendEntriesArgs struct {
	Term int
	LeaderId int
	PrevLogIndex int
	PrevLogTerm int
	Entries []interface{}
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term int
	Success bool
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply){
	//heartbeat 
	rf.mx.Lock()
	defer rf.mx.Unlock()
	
	//stale leader
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return 
	}

	//leader heartbeat
	rf.heartbeatTime = time.Now()
	rf.leaderIndex = args.LeaderId
	rf.votedFor = NotVoted
	
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
	}
	if len(args.Entries) == 0 {
		DPrintf("%v got heartbeat from %v", rf.me, args.LeaderId)
		return
	}

	//leader append entry
	reply.Term = rf.currentTerm
	reply.Success = true
	if args.Term < rf.currentTerm || rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Success = false
	}

	//TO-DO append non-exist entries
}
//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mx.Lock()
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
	}
	reply.Term = rf.currentTerm
	loglen := len(rf.log)

	if rf.votedFor == NotVoted || rf.votedFor == args.CandidateId {
		if args.Term < rf.currentTerm {
			reply.VoteGranted = false
		} else if loglen == 0 || args.LastLogTerm > rf.log[loglen - 1].Term ||
			(args.LastLogTerm == rf.log[loglen - 1].Term && 
			args.LastLogIndex >= len(rf.log) - 1) {
			
			rf.votedFor = args.CandidateId
			reply.VoteGranted = true
		} else {
			reply.VoteGranted = false
		}

	} else {
		reply.VoteGranted = false
	}
	rf.mx.Unlock()
	DPrintf("candidate %v reqvote to %v , got %v, candidate term %v, me term %v ,votedfor %v, argindex %v meindex %v", args.CandidateId, rf.me, 
	reply.VoteGranted, args.LastLogTerm, rf.currentTerm, rf.votedFor, args.LastLogIndex, len(rf.log) - 1)
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool{
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}
//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply, ch chan int) bool {

	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	ch <- 1
	return ok
}


//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).


	return index, term, isLeader
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	
	rf.currentTerm = 1
	rf.log = []LogEntry{}
	rf.ch = applyCh
	rf.votedFor = NotVoted
	// Your initialization code here (2A, 2B, 2C).

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	go func(){
			//heartbeat goroutine
			for{
				// 1s = 1000 milliseconds 1000ms
				//leader send heartbeats, at most 10 heartbeats/s
				rf.mx.Lock()
	
				if rf.me == rf.leaderIndex {
					DPrintf("%v hearting ", rf.me)
					for i := 0; i < len(rf.peers); i++ {
						if i == me {
						continue
						}
				
						go func(i int){ 
							args := AppendEntriesArgs{}
							args.Term = rf.currentTerm
							args.LeaderId = rf.me
							
							rf.sendAppendEntries(i, &args, &AppendEntriesReply{})
						}(i)
					}
				}
				rf.mx.Unlock() 			
				time.Sleep(100 * time.Millisecond)
			}
	}()
	

	go func(){
		//election goroutine 
		for {
			time.Sleep(time.Second)

			if rf.me != rf.leaderIndex && time.Now().After(rf.heartbeatTime.Add(time.Millisecond * 150)) {
				
				DPrintf("%v start election", rf.me)
			
				//timeout 250 ~ 400 Millisecond
				//random timeout 250 - 400 ms
				isTimeout := make(chan bool)

				go func(isTimeout chan bool) {
					timeout := rand.Int() % 150 + 1 + 250 
					time.Sleep(time.Duration(timeout) * time.Millisecond)
					isTimeout <- true
				}(isTimeout)
			
				majority := (len(rf.peers) - 1) / 2 + 1
				voteGot := 0
				var mutex sync.Mutex 
				
				//start election poll
				for i := 0; i < len(rf.peers); i++ {			
					args := RequestVoteArgs{}
					args.Term = rf.currentTerm
					args.CandidateId = rf.me
					
					if len(rf.log) != 0 {
						args.LastLogIndex = len(rf.log) - 1
						args.LastLogTerm = rf.log[len(rf.log) - 1].Term
					} else {
						args.LastLogIndex = 0
						args.LastLogTerm = 1
					}
		
					reply := RequestVoteReply{}
					
					go func(i int){
						ch := make(chan int)
						go func(){
							rf.sendRequestVote(i, &args, &reply, ch)
						}()
						

						//for synchronization
						<- ch
						if reply.VoteGranted == true {		
							mutex.Lock()
							voteGot = voteGot + 1
							if voteGot >= majority && rf.me != rf.leaderIndex {

								rf.leaderIndex = rf.me
								rf.currentTerm = rf.currentTerm + 1 
								DPrintf("term %v got leader %v", rf.currentTerm, rf.me)
							}
							mutex.Unlock()
						}
					}(i)
				}
				DPrintf("%v server %v request sent", rf.me, len(rf.peers))
				<- isTimeout
				DPrintf("%v election finished with %v got ", rf.me, voteGot)
			}
			
			rf.mx.Lock()
			rf.votedFor = NotVoted
			rf.mx.Unlock()
		}		
	}()

	return rf
}
