package notify

type NotifyMessage struct {
	// Source distinguishes notification origin: "idle" (session idle detection)
	// or "cli" (manual notification via tlive notify).
	Source string

	// Fields for idle notifications (Source="idle")
	SessionID   string
	Command     string
	Pid         int
	Duration    string
	LastOutput  string
	WebURL      string
	IdleSeconds int
	Confidence  string // "high" or "low"

	// Fields for CLI notifications (Source="cli")
	Type    string // done, confirm, error, progress
	Message string
	Context string
}

type Notifier interface {
	Send(msg *NotifyMessage) error
}

type MultiNotifier struct {
	notifiers []Notifier
}

func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

func (m *MultiNotifier) Send(msg *NotifyMessage) error {
	var firstErr error
	for _, n := range m.notifiers {
		if err := n.Send(msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
