package tasks

// Task types.
const (
	TypeExample   = "tasks:example"
	TypeDBCleanup = "db:cleanup"
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
