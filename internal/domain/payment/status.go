package payment

type Status string

const (
	StatusPending           Status = "pending"
	StatusAuthorized        Status = "authorized"         // autorizado no PSP, aguardando captura (fluxo manual)
	StatusCompleted         Status = "completed"          // capturado/liquidado
	StatusFailed            Status = "failed"             // recusado pelo PSP
	StatusRefunded          Status = "refunded"           // estornado integralmente
	StatusPartiallyRefunded Status = "partially_refunded" // estornado em parte
)

func (s Status) IsValid() bool {
	switch s {
	case StatusPending,
		StatusAuthorized,
		StatusCompleted,
		StatusFailed,
		StatusRefunded,
		StatusPartiallyRefunded:
		return true
	default:
		return false
	}
}
