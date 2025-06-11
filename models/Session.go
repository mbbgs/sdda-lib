package models

import "sync"


// Define Session structure
type Session struct {
	mu          sync.RWMutex
	IsConnected bool
	Peers       map[string]Peer
}


// CreateSession initializes a new session
func CreateSession() *Session {
	return &Session{
		mu:          sync.RWMutex{},
		Peers:       make(map[string]Peer, 2),
		IsConnected: false,
	}
}

// AddPeer adds a single peer to the session
func (s *Session) AddPeer(peer Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Peers[peer.PeerID] = peer
}

// AddPeers adds multiple peers to the session
func (s *Session) AddPeers(peers []Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, peer := range peers {
		s.Peers[peer.PeerID] = peer
	}
}

// RemovePeer removes a peer from the session by PeerID
func (s *Session) RemovePeer(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Peers, peerID)
}

// GetPeer retrieves a peer by PeerID
func (s *Session) GetPeer(peerID string) (Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peer, found := s.Peers[peerID]
	return peer, found
}

// ListPeers returns a list of all peers in the session
func (s *Session) ListPeers() []Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peers := make([]Peer, 0, len(s.Peers))
	for _, peer := range s.Peers {
		peers = append(peers, peer)
	}
	return peers
}