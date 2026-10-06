package agents

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

var conversationID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// ValidConversationID excludes paths, options and shell syntax from stored IDs.
func ValidConversationID(id string) bool { return conversationID.MatchString(id) }

// NewConversationID creates a UUID for agents that accept caller-owned IDs.
func NewConversationID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}

// ResumeArgs selects an exact conversation, never the most recent session.
func ResumeArgs(agent, id string) ([]string, error) {
	if !ValidConversationID(id) {
		return nil, fmt.Errorf("invalid agent conversation ID")
	}
	switch agent {
	case "claude", "grok":
		return []string{"--resume", id}, nil
	case "codex":
		return []string{"resume", id}, nil
	case "opencode", "kilo":
		return []string{"--session", id}, nil
	default:
		return nil, fmt.Errorf("unsupported coding agent: %s", agent)
	}
}
