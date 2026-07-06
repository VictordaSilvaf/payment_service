package payment

type Status string

const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusCompleted, StatusFailed:
		return true
	default:
		return false
	}
}
