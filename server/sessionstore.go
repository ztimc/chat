/******************************************************************************
 *
 *  Description:
 *
 *  Session management.
 *
 *****************************************************************************/

package main

import (
	"container/list"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tinode/chat/pbx"
	"github.com/tinode/chat/server/store"
	"github.com/tinode/chat/server/store/types"
)

// SessionStore holds live sessions. Long polling sessions are stored in a linked list with
// most recent sessions on top. In addition all sessions are stored in a map indexed by session ID.
type SessionStore struct {
	lock sync.Mutex

	// Support for long polling sessions: a list of sessions sorted by last access time.
	// Needed for cleaning abandoned sessions.
	lru      *list.List
	lifeTime time.Duration

	// All sessions indexed by session ID
	sessCache map[string]*Session
}

// NewSession creates a new session and saves it to the session store.
func (ss *SessionStore) NewSession(conn interface{}, sid string) (*Session, int) {
	var s Session

	s.sid = sid

	switch c := conn.(type) {
	case *websocket.Conn:
		s.proto = WEBSOCK
		s.ws = c
	case http.ResponseWriter:
		s.proto = LPOLL
		// no need to store c for long polling, it changes with every request
	case *ClusterNode:
		s.proto = CLUSTER
		s.clnode = c
	case pbx.Node_MessageLoopServer:
		s.proto = GRPC
		s.grpcnode = c
	default:
		s.proto = NONE
	}

	if s.proto != NONE {
		s.subs = make(map[string]*Subscription)
		s.send = make(chan interface{}, 256) // buffered
		s.stop = make(chan interface{}, 1)   // Buffered by 1 just to make it non-blocking
		s.detach = make(chan string, 64)     // buffered
	}

	s.lastTouched = time.Now()
	if s.sid == "" {
		s.sid = store.GetUidString()
	}

	ss.lock.Lock()

	ss.sessCache[s.sid] = &s
	count := len(ss.sessCache)
	var expired []*Session
	if s.proto == LPOLL {
		// Only LP sessions need to be sorted by last active
		s.lpTracker = ss.lru.PushFront(&s)

		// Remove expired sessions
		expire := s.lastTouched.Add(-ss.lifeTime)
		for elem := ss.lru.Back(); elem != nil; elem = ss.lru.Back() {
			sess := elem.Value.(*Session)
			if sess.lastTouched.Before(expire) {
				ss.lru.Remove(elem)
				delete(ss.sessCache, sess.sid)
				expired = append(expired, sess)
			} else {
				break // don't need to traverse further
			}
		}
		count -= len(expired)
	}
	ss.lock.Unlock()

	for _, sess := range expired {
		// This locks the session. Thus cleaning up outside of the
		// sessionStore lock. Otherwise deadlock.
		sess.cleanUp(true)
	}

	statsSet("LiveSessions", int64(len(ss.sessCache)))
	statsInc("TotalSessions", 1)

	return &s, count
}

// Get fetches a session from store by session ID.
func (ss *SessionStore) Get(sid string) *Session {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	if sess := ss.sessCache[sid]; sess != nil {
		if sess.proto == LPOLL {
			ss.lru.MoveToFront(sess.lpTracker)
			sess.lastTouched = time.Now()
		}

		return sess
	}

	return nil
}

// Delete removes session from store.
func (ss *SessionStore) Delete(s *Session) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	delete(ss.sessCache, s.sid)
	if s.proto == LPOLL {
		ss.lru.Remove(s.lpTracker)
	}

	statsSet("LiveSessions", int64(len(ss.sessCache)))
}

// Shutdown terminates sessionStore. No need to clean up.
// Don't send to clustered sessions, their servers are not being shut down.
func (ss *SessionStore) Shutdown() {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	shutdown := NoErrShutdown(types.TimeNow())
	for _, s := range ss.sessCache {
		if s.stop != nil && s.proto != CLUSTER {
			s.stop <- s.serialize(shutdown)
		}
	}

	log.Printf("SessionStore shut down, sessions terminated: %d", len(ss.sessCache))
}

// EvictUser terminates all sessions of a given user.
func (ss *SessionStore) EvictUser(uid types.Uid, skipSid string) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	evicted := NoErrEvicted("", "", types.TimeNow())
	for _, s := range ss.sessCache {
		if s.uid == uid && s.stop != nil && s.sid != skipSid {
			s.stop <- s.serialize(evicted)
			delete(ss.sessCache, s.sid)
			if s.proto == LPOLL {
				ss.lru.Remove(s.lpTracker)
			}
		}
	}

	statsSet("LiveSessions", int64(len(ss.sessCache)))
}

// NewSessionStore initializes a session store.
func NewSessionStore(lifetime time.Duration) *SessionStore {
	ss := &SessionStore{
		lru:      list.New(),
		lifeTime: lifetime,

		sessCache: make(map[string]*Session),
	}

	statsRegisterInt("LiveSessions")
	statsRegisterInt("TotalSessions")

	return ss
}
