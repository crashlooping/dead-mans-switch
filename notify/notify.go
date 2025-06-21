package notify

// Notifier is the interface for all notification channels.
type Notifier interface {
	Notify(subject, message string) error
}

// Registry for pluggable notifiers
var notifiers = map[string]func(map[string]string) Notifier{}

// Register a notifier constructor by type name
func Register(name string, constructor func(map[string]string) Notifier) {
	notifiers[name] = constructor
}

// CreateNotifier creates a notifier from config
func CreateNotifier(channelType string, props map[string]string) Notifier {
	if ctor, ok := notifiers[channelType]; ok {
		return ctor(props)
	}
	return nil
}
