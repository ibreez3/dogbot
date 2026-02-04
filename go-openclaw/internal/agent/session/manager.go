package session

import (
	"sync"
	"time"
)

// Manager manages multiple sessions
type Manager struct {
	sessions map[string]*Session
	lock     sync.RWMutex
}

// NewManager creates a new session manager
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// GetOrCreate gets or creates a session by ID
func (m *Manager) GetOrCreate(sessionID string) (*Session, error) {
	m.lock.RLock()
	session, ok := m.sessions[sessionID]
	m.lock.RUnlock()

	if ok {
		return session, nil
	}

	// Create new session
	session = &Session{
		ID:         sessionID,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Status:     "active",
		Messages:   make([]*Message, 0),
		Metadata:   make(map[string]interface{}),
	}

	m.lock.Lock()
	m.sessions[sessionID] = session
	m.lock.Unlock()

	return session, nil
}

// Get retrieves a session by ID
func (m *Manager) Get(sessionID string) (*Session, bool) {
	m.lock.RLock()
	session, ok := m.sessions[sessionID]
	m.lock.RUnlock()
	return session, ok
}

// Delete removes a session
func (m *Manager) Delete(sessionID string) {
	m.lock.Lock()
	delete(m.sessions, sessionID)
	m.lock.Unlock()
}

// Cleanup removes inactive sessions older than specified duration
func (m *Manager) Cleanup(maxAge time.Duration) int {
	m.lock.Lock()
	defer m.lock.Unlock()

	now := time.Now()
	removed := 0

	for id, session := range m.sessions {
		if now.Sub(session.LastActive) > maxAge {
			delete(m.sessions, id)
			removed++
		}
	}

	return removed
}
