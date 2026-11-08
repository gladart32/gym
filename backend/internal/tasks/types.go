package tasks

// Task types.
const (
	TypeExample   = "tasks:example"
	TypeDBCleanup = "db:cleanup"
	TypeSendEmail = "tasks:send_email"
)

// Queue names.
const (
	QueueCritical    = "critical"
	QueueDefault     = "default"
	QueueMaintenance = "maintenance"
)

// ExamplePayload is the payload of the example task.
type ExamplePayload struct {
	Message string `json:"message"`
}

// SendEmailPayload is the payload of the email task.
type SendEmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}
