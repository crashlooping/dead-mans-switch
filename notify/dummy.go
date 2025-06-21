package notify

import (
	"fmt"
)

type DummyNotifier struct {
	Target string
}

func (d *DummyNotifier) Notify(subject, message string) error {
	fmt.Printf("[DUMMY] To: %s | %s: %s\n", d.Target, subject, message)
	return nil
}

func NewDummyNotifier(props map[string]string) Notifier {
	return &DummyNotifier{Target: props["to"]}
}

func init() {
	Register("dummy", NewDummyNotifier)
}
