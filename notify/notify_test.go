package notify

import "testing"

type testNotifier struct {
	called bool
}

func (t *testNotifier) Notify(subject, message string) error {
	t.called = true
	return nil
}

func TestRegisterAndCreateNotifier(t *testing.T) {
	Register("test", func(props map[string]string) Notifier {
		return &testNotifier{}
	})
	n := CreateNotifier("test", nil)
	if n == nil {
		t.Fatal("notifier not created")
	}
	tn, ok := n.(*testNotifier)
	if !ok {
		t.Fatal("wrong type")
	}
	n.Notify("subj", "msg")
	if !tn.called {
		t.Error("Notify not called")
	}
}
