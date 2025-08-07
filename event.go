package workerpool

type Event int

const (
	EventPushTask Event = iota
	EventStartTask
	EventEndTask
	EventStartWorker
)

type EventHandler func(e Event, a ...any)
