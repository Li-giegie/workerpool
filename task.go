package workerpool

type Task interface {
	Id() any
	Call()
	State() TaskState
	Out() []any
	Err() error
}

type TaskState int

func (s TaskState) String() string {
	switch s {
	case TaskStateWait:
		return "wait"
	case TaskStateIng:
		return "ing"
	case TaskStateDone:
		return "done"
	default:
		return "unknown"
	}
}

const (
	TaskStateWait = iota
	TaskStateIng
	TaskStateDone
)

func NewTask(id any, fn any, in ...any) Task {
	return &task{
		id: id,
		in: in,
		fn: fn,
	}
}

type task struct {
	id    any
	in    []any
	out   []any
	state TaskState
	fn    any
	err   error
}

func (t *task) Id() any {
	return t.id
}

func (t *task) State() TaskState {
	return t.state
}

func (t *task) Call() {
	t.state = TaskStateIng
	defer func() {
		t.state = TaskStateDone
	}()
	fn, ok := t.fn.(func())
	if ok {
		fn()
		return
	}
	if t.in == nil {
		t.out, t.err = call(t.fn)
	} else {
		t.out, t.err = call(t.fn, t.in...)
	}
}

func (t *task) Out() []any {
	return t.out
}

func (t *task) Err() error {
	return t.err
}
