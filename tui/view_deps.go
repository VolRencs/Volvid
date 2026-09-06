package tui

type depStatusRow struct {
	Label string
	Value string
}

type depState string

const (
	depStateActive    depState = "active"
	depStateMissing   depState = "missing"
	depStateNotActive depState = "not_active"
	depStateAvailable depState = "available"
	depStateChecking  depState = "checking"
)
