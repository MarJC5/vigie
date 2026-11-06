package model

import "errors"

// ErrInvalidAssistantAccess refuses a level of the assistants Vigie does not know.
var ErrInvalidAssistantAccess = errors.New("the assistants are off, read or act")

// AssistantAccess is what an assistant may do through vigie mcp (ADR-0043, point 3).
type AssistantAccess string

// The three levels, each allowing what the one before allows.
const (
	AssistantOff  AssistantAccess = "off"  // no tool, the default
	AssistantRead AssistantAccess = "read" // read what Vigie knows, and scan
	AssistantAct  AssistantAccess = "act"  // also update, copy and ignore, each confirmed
)

// Valid tells a level Vigie knows.
func (a AssistantAccess) Valid() bool {
	return a == AssistantOff || a == AssistantRead || a == AssistantAct
}

// Allows tells whether this level allows the tools of another: act allows read's, nothing allows
// off's.
func (a AssistantAccess) Allows(level AssistantAccess) bool {
	switch level {
	case AssistantRead:
		return a == AssistantRead || a == AssistantAct
	case AssistantAct:
		return a == AssistantAct
	}
	return false
}
