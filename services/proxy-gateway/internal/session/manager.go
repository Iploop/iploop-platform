package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"

	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/nodepool"
)

type SessionManager struct {
	rdb         *redis.Client
	nodePool    *nodepool.NodePool
	wsNodePool  *nodepool.WebSocketNodePool
	logger      *logrus.Entry
	sessions    sync.Map // In-memory cache for active sessions
}

type Session struct {
	ID              string                 `json:"id"`
	CustomerID      string                 `json:"customer_id"`
	Type            string                 `json:"type"` // sticky, rotating, per-request
	CreatedAt       time.Time             `json:"created_at"`
	LastUsed        time.Time             `json:"last_used"`
	ExpiresAt       time.Time             `json:"expires_at"`
	
	// Node assignment
	CurrentNodeID   string                 `json:"current_node_id"`
	CurrentNodeIP   string                 `json:"current_node_ip"`
	NodeHistory     []NodeAssignment       `json:"node_history"`
	
	// Rotation settings
	RotateMode      string                 `json:"rotate_mode"` // request, time, manual, ip-change
	RotateInterval  time.Duration         `json:"rotate_interval"`
	LastRotation    time.Time             `json:"last_rotation"`
	RequestCount    int64                 `json:"request_count"`
	
	// Geographic preferences
	Country         string                 `json:"country"`
	City            string                 `json:"city"`
	ASN             int                   `json:"asn"`
	
	// Performance requirements
	MinSpeed        int                   `json:"min_speed"`
	MaxLatency      int                   `json:"max_latency"`
	
	// Usage tracking
	BytesTransferred int64                `json:"bytes_transferred"`
	SuccessfulRequests int64              `json:"successful_requests"`
	FailedRequests   int64                `json:"failed_requests"`
	
	// Session state
	Headers         map[string]string      `json:"headers"`
	UserAgent       string                `json:"user_agent"`
	Profile         string                `json:"profile"`
	
	mutex           sync.RWMutex          `json:"-"`
}

type NodeAssignment struct {
	NodeID    string    `json:"node_id"`
	NodeIP    string    `json:"node_ip"`
	AssignedAt time.Time `json:"assigned_at"`
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	BytesUsed int64     `json:"bytes_used"`
	RequestCount int64  `json:"request_count"`
}

func NewSessionManager(rdb *redis.Client, nodePool *nodepool.NodePool, wsNodePool *nodepool.WebSocketNodePool, logger *logrus.Entry) *SessionManager {
	return &SessionManager{
		rdb:        rdb,
		nodePool:   nodePool,
		wsNodePool: wsNodePool,
		logger:     logger.WithField("component", "session-manager"),
	}
}

func (sm *SessionManager) GetOrCreateSession(auth *auth.EnhancedProxyAuth) (*Session, error) {
	sessionKey := sm.generateSessionKey(auth)
	
	// Try to get existing session
	if session := sm.getFromCache(sessionKey); session != nil {
		session.mutex.Lock()
		session.LastUsed = time.Now()
		session.mutex.Unlock()
		
		// Check if rotation is needed
		if sm.shouldRotate(session) {
			if err := sm.rotateNode(session); err != nil {
				sm.logger.Warnf("Failed to rotate node for session %s: %v", session.ID, err)
			}
		}
		
		return session, nil
	}
	
	// Try Redis
	session, err := sm.getFromRedis(sessionKey)
	if err == nil {
		sm.sessions.Store(sessionKey, session)
		return session, nil
	}
	
	// Create new session
	return sm.createNewSession(auth, sessionKey)
}

func (sm *SessionManager) createNewSession(auth *auth.EnhancedProxyAuth, sessionKey string) (*Session, error) {
	session := &Session{
		ID:               auth.SessionID,
		CustomerID:       auth.Customer.ID,
		Type:             auth.SessionType,
		CreatedAt:        time.Now(),
		LastUsed:         time.Now(),
		ExpiresAt:        time.Now().Add(auth.Lifetime),
		RotateMode:       auth.RotateMode,
		RotateInterval:   auth.RotateInterval,
		Country:          auth.Country,
		City:             auth.City,
		ASN:              auth.ASN,
		MinSpeed:         auth.MinSpeed,
		MaxLatency:       auth.MaxLatency,
		Headers:          auth.Headers,
		UserAgent:        auth.UserAgent,
		Profile:          auth.Profile,
		NodeHistory:      make([]NodeAssignment, 0),
	}
	
	// Assign initial node
	if err := sm.assignNode(session); err != nil {
		return nil, fmt.Errorf("failed to assign node: %v", err)
	}
	
	// Save to Redis
	if err := sm.saveToRedis(sessionKey, session); err != nil {
		sm.logger.Warnf("Failed to save session to Redis: %v", err)
	}
	
	// Cache in memory
	sm.sessions.Store(sessionKey, session)
	
	// Schedule cleanup
	sm.scheduleCleanup(sessionKey, auth.Lifetime)
	
	sm.logger.Infof("Created new session %s for customer %s", session.ID, session.CustomerID)
	return session, nil
}

func (sm *SessionManager) assignNode(session *Session) error {
	selection := &nodepool.NodeSelection{
		Country:    session.Country,
		City:       session.City,
		ASN:        session.ASN,
		MinSpeed:   session.MinSpeed,
		MaxLatency: session.MaxLatency,
		SessionID:  session.ID,
	}
	
	node, err := sm.nodePool.SelectNode(selection)
	if err != nil {
		return fmt.Errorf("no suitable nodes available: %v", err)
	}
	
	session.mutex.Lock()
	defer session.mutex.Unlock()
	
	// Release previous node if any
	if session.CurrentNodeID != "" {
		sm.releaseCurrentNode(session)
	}
	
	// Assign new node
	session.CurrentNodeID = node.ID
	session.CurrentNodeIP = node.IPAddress
	session.LastRotation = time.Now()
	
	// Add to history
	assignment := NodeAssignment{
		NodeID:     node.ID,
		NodeIP:     node.IPAddress,
		AssignedAt: time.Now(),
	}
	session.NodeHistory = append(session.NodeHistory, assignment)
	
	sm.logger.Debugf("Assigned node %s (%s) to session %s", node.ID, node.IPAddress, session.ID)
	return nil
}

func (sm *SessionManager) releaseCurrentNode(session *Session) {
	if len(session.NodeHistory) > 0 {
		lastAssignment := &session.NodeHistory[len(session.NodeHistory)-1]
		if lastAssignment.ReleasedAt == nil {
			now := time.Now()
			lastAssignment.ReleasedAt = &now
			
			sm.nodePool.ReleaseNode(session.CurrentNodeID)
			sm.logger.Debugf("Released node %s from session %s", session.CurrentNodeID, session.ID)
		}
	}
}

func (sm *SessionManager) shouldRotate(session *Session) bool {
	session.mutex.RLock()
	defer session.mutex.RUnlock()
	
	switch session.RotateMode {
	case "request":
		return true // Rotate on every request
	case "time":
		return time.Since(session.LastRotation) >= session.RotateInterval
	case "manual":
		return false // Only rotate when explicitly requested
	case "ip-change":
		// Would need to check if target IP changed (complex logic)
		return false
	default:
		return false
	}
}

func (sm *SessionManager) rotateNode(session *Session) error {
	sm.logger.Debugf("Rotating node for session %s", session.ID)
	return sm.assignNode(session)
}

func (sm *SessionManager) RecordUsage(sessionID string, bytesUsed int64, success bool) {
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	
	if sessionData, exists := sm.sessions.Load(sessionKey); exists {
		session := sessionData.(*Session)
		session.mutex.Lock()
		
		session.BytesTransferred += bytesUsed
		session.RequestCount++
		
		if success {
			session.SuccessfulRequests++
		} else {
			session.FailedRequests++
		}
		
		// Update current node assignment usage
		if len(session.NodeHistory) > 0 {
			lastAssignment := &session.NodeHistory[len(session.NodeHistory)-1]
			if lastAssignment.ReleasedAt == nil {
				lastAssignment.BytesUsed += bytesUsed
				lastAssignment.RequestCount++
			}
		}
		
		session.mutex.Unlock()
		
		// Update Redis asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			sessionData, _ := json.Marshal(session)
			sm.rdb.Set(ctx, sessionKey, sessionData, session.ExpiresAt.Sub(time.Now()))
		}()
	}
}

func (sm *SessionManager) GetSessionStats(sessionID string) (*Session, error) {
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	
	if sessionData, exists := sm.sessions.Load(sessionKey); exists {
		session := sessionData.(*Session)
		session.mutex.RLock()
		defer session.mutex.RUnlock()
		
		// Return a copy to prevent external modifications
		return sm.copySession(session), nil
	}
	
	// Try Redis
	return sm.getFromRedis(sessionKey)
}

func (sm *SessionManager) TerminateSession(sessionID string) error {
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	
	if sessionData, exists := sm.sessions.LoadAndDelete(sessionKey); exists {
		session := sessionData.(*Session)
		session.mutex.Lock()
		sm.releaseCurrentNode(session)
		session.mutex.Unlock()
	}
	
	// Remove from Redis
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	sm.rdb.Del(ctx, sessionKey)
	
	sm.logger.Infof("Terminated session %s", sessionID)
	return nil
}

func (sm *SessionManager) generateSessionKey(auth *auth.EnhancedProxyAuth) string {
	if auth.SessionID != "" {
		return fmt.Sprintf("session:%s", auth.SessionID)
	}
	
	// Generate based on customer and parameters
	data := fmt.Sprintf("%s_%s_%s_%s_%d", 
		auth.Customer.ID, auth.Country, auth.City, auth.SessionType, time.Now().Unix())
	
	hasher := sha256.New()
	hasher.Write([]byte(data))
	sessionID := hex.EncodeToString(hasher.Sum(nil))[:16]
	
	return fmt.Sprintf("session:%s", sessionID)
}

func (sm *SessionManager) getFromCache(sessionKey string) *Session {
	if sessionData, exists := sm.sessions.Load(sessionKey); exists {
		session := sessionData.(*Session)
		
		// Check if expired
		if time.Now().After(session.ExpiresAt) {
			sm.sessions.Delete(sessionKey)
			return nil
		}
		
		return session
	}
	return nil
}

func (sm *SessionManager) getFromRedis(sessionKey string) (*Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	data, err := sm.rdb.Get(ctx, sessionKey).Result()
	if err != nil {
		return nil, err
	}
	
	var session Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	
	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		sm.rdb.Del(ctx, sessionKey)
		return nil, fmt.Errorf("session expired")
	}
	
	return &session, nil
}

func (sm *SessionManager) saveToRedis(sessionKey string, session *Session) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	sessionData, err := json.Marshal(session)
	if err != nil {
		return err
	}
	
	return sm.rdb.Set(ctx, sessionKey, sessionData, session.ExpiresAt.Sub(time.Now())).Err()
}

func (sm *SessionManager) scheduleCleanup(sessionKey string, lifetime time.Duration) {
	time.AfterFunc(lifetime, func() {
		sm.sessions.Delete(sessionKey)
		sm.logger.Debugf("Cleaned up expired session %s", sessionKey)
	})
}

func (sm *SessionManager) copySession(session *Session) *Session {
	copy := *session
	copy.Headers = make(map[string]string)
	for k, v := range session.Headers {
		copy.Headers[k] = v
	}
	copy.NodeHistory = make([]NodeAssignment, len(session.NodeHistory))
	copy(copy.NodeHistory, session.NodeHistory)
	return &copy
}

// Cleanup runs periodically to remove expired sessions
func (sm *SessionManager) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				sm.cleanupExpiredSessions()
			}
		}
	}()
}

func (sm *SessionManager) cleanupExpiredSessions() {
	now := time.Now()
	
	sm.sessions.Range(func(key, value interface{}) bool {
		session := value.(*Session)
		if now.After(session.ExpiresAt) {
			sm.sessions.Delete(key)
			
			session.mutex.Lock()
			sm.releaseCurrentNode(session)
			session.mutex.Unlock()
			
			sm.logger.Debugf("Cleaned up expired session %s", session.ID)
		}
		return true
	})
}