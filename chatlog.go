package main

import (
	"sync"
)

// SafeChatLog is safe to use concurrently.
type SafeRoomLog struct {
	v   map[string][][]byte
	sync.RWMutex
}

type UserKey struct {
	Userid, Roomid string
}

// SafeUserLog is safe to use concurrently.
type SafeUserLog struct {
	v   map[UserKey]int
	sync.RWMutex
}
