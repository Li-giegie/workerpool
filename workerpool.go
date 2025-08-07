package workerpool

import (
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
)

// WorkerPool 任务执行池
type WorkerPool interface {
	// Push fn：任意的函数签名，in：函数的入参，把一个待执行的函数，推入待执行管道，如果管道满了，则会放入等待队列，直到执行管道有容量时在执行，如果等待队列超过容量则返回ErrOverflow错误（不会陷入阻塞），
	Push(fn any, in ...any) error
	// PushTask workerPool.NewTask
	PushTask(t Task) error
	PushWithId(id, fn any, in ...any) error
	// Run 阻塞启动
	Run() error
	// Reboot 重启
	Reboot() error
	// Stop 终止
	Stop()
	// AddListener 添加事件侦听方法
	AddListener(e Event, fn EventHandler, isAsync ...bool)
	// RemoveListener 移除某一类事件
	RemoveListener(e Event)
	// WaitTaskNum 待执行任务数量
	WaitTaskNum() int
	// Free 可用任务管道容量
	Free() int
	// State 获取工作池状态
	State() WorkerPoolState
}

// NewWorkerPool 创建工作协程池，numWorker：工作协程数、numTaskQueue：存放任务的管道缓冲区容量，
// optNumWaitQueue：[可选] 默认不限制容量，当存放任务的管道缓冲区容量满后，新添加的任务会被放入等待队列中，
// 当numWaitQueue < 0 时不限制等待队列的容量；
// 当numWaitQueue == 0 时禁用等待队列直接写入执行管道，如果执行管道容量满，则返回ErrOverflow错误；
// 当numWaitQueue > 0 时等待队列有可用容量则放入等待队列，否则返回返回ErrOverflow错误；
func NewWorkerPool(numWorker, numTaskQueue int, optNumWaitQueue ...int) WorkerPool {
	if numWorker < 1 {
		panic("numWorker must be greater than 0")
	}
	if numTaskQueue < 1 {
		panic("numTaskQueue must be greater than 0")
	}
	numWaitQueue := -1
	if len(optNumWaitQueue) > 0 {
		numWaitQueue = optNumWaitQueue[0]
	}
	return &workerPool{
		numWorker:    numWorker,
		numTaskQueue: numTaskQueue,
		numWaitQueue: numWaitQueue,
	}
}

type WorkerPoolState uint32

func (s WorkerPoolState) String() string {
	switch s {
	case WorkerPoolStateRunning:
		return "running"
	case WorkerPoolStateStopped:
		return "stopped"
	default:
		return "invalid state"
	}
}

const (
	WorkerPoolStateStopped WorkerPoolState = iota
	WorkerPoolStateRunning
)

type workerPool struct {
	numWorker    int
	numTaskQueue int
	numWaitQueue int
	lock         sync.Mutex
	wg           sync.WaitGroup
	taskQueue    chan Task
	waitQueue    []Task
	eventMap     sync.Map
	state        WorkerPoolState
}

func (w *workerPool) Push(fn any, in ...any) error {
	return w.PushTask(NewTask(nil, fn, in...))
}

func (w *workerPool) PushWithId(id, fn any, in ...any) error {
	return w.PushTask(NewTask(id, fn, in...))
}

var ErrOverflow = errors.New("wait queue overflow")

func (w *workerPool) PushTask(t Task) error {
	if atomic.LoadUint32((*uint32)(&w.state)) == uint32(WorkerPoolStateStopped) {
		return errors.New("push err: WorkerPool has stopped")
	}
	w.lock.Lock()
	if len(w.taskQueue) < cap(w.taskQueue) {
		w.taskQueue <- t
	} else {
		if w.numWaitQueue >= 0 && len(w.waitQueue) >= w.numWaitQueue {
			return ErrOverflow
		}
		w.waitQueue = append(w.waitQueue, t)
	}
	w.lock.Unlock()
	if e, ok := w.eventMap.Load(EventPushTask); ok {
		for _, handler := range e.([]EventHandler) {
			handler(EventPushTask, t)
		}
	}
	return nil
}

func (w *workerPool) Run() error {
	if w.numWorker < 1 {
		return errors.New("numWorker must be greater than 0")
	}
	if w.numTaskQueue < 1 {
		return errors.New("numTaskQueue must be greater than 0")
	}
	if !atomic.CompareAndSwapUint32((*uint32)(&w.state), uint32(WorkerPoolStateStopped), uint32(WorkerPoolStateRunning)) {
		return errors.New("run err: is already running")
	}
	w.taskQueue = make(chan Task, w.numTaskQueue)
	for i := 0; i < w.numWorker; i++ {
		w.wg.Add(1)
		go w.Worker(i)
	}
	w.wg.Wait()
	return nil
}

func (w *workerPool) Reboot() error {
	if atomic.LoadUint32((*uint32)(&w.state)) == uint32(WorkerPoolStateRunning) {
		w.Stop()
	}
	return w.Run()
}

func (w *workerPool) Stop() {
	if atomic.CompareAndSwapUint32((*uint32)(&w.state), uint32(WorkerPoolStateRunning), uint32(WorkerPoolStateStopped)) {
		w.lock.Lock()
		close(w.taskQueue)
		w.waitQueue = nil
		w.lock.Unlock()
		w.wg.Wait()
	}
}

func (w *workerPool) Worker(id int) {
	if e, ok := w.eventMap.Load(EventStartWorker); ok {
		for _, handler := range e.([]EventHandler) {
			handler(EventStartWorker, id)
		}
	}
	defer w.wg.Done()
	for t := range w.taskQueue {
		if atomic.LoadUint32((*uint32)(&w.state)) == uint32(WorkerPoolStateStopped) {
			continue
		}
		if e, ok := w.eventMap.Load(EventStartTask); ok {
			for _, handler := range e.([]EventHandler) {
				handler(EventStartTask, t)
			}
		}
		t.Call()
		if e, ok := w.eventMap.Load(EventEndTask); ok {
			for _, handler := range e.([]EventHandler) {
				handler(EventEndTask, t)
			}
		}
		w.lock.Lock()
		if len(w.waitQueue) > 0 && len(w.taskQueue) < cap(w.taskQueue) {
			w.taskQueue <- w.waitQueue[0]
			w.waitQueue = w.waitQueue[1:]
		}
		w.lock.Unlock()
	}
}

func (w *workerPool) AddListener(e Event, fn EventHandler, isAsync ...bool) {
	var list []EventHandler
	v, ok := w.eventMap.Load(e)
	if ok {
		list = v.([]EventHandler)
	}
	if len(isAsync) == 1 && isAsync[0] {
		list = append(list, func(e Event, a ...any) { go fn(e, a...) })
		return
	}
	list = append(list, fn)
	w.eventMap.Store(e, list)
}

func (w *workerPool) RemoveListener(e Event) {
	w.eventMap.Delete(e)
}

func (w *workerPool) WaitTaskNum() int {
	return len(w.waitQueue) + len(w.taskQueue)
}

func (w *workerPool) Free() int {
	return cap(w.taskQueue) - len(w.taskQueue)
}

func (w *workerPool) State() WorkerPoolState {
	return w.state
}

// call 调用方法，args 方法的入参，返回值error决定本次是否调用成功，[]any切片列表:为方法的实际返回值
func call(fn any, args ...any) ([]any, error) {
	rv := reflect.ValueOf(fn)
	if !rv.IsValid() {
		return nil, errors.New("fn is invalid")
	}
	if rv.Kind() != reflect.Func {
		return nil, errors.New("fn not is a function")
	}
	rt := rv.Type()
	var in = make([]reflect.Value, 0, rt.NumIn())
	var out []reflect.Value
	numIn := rt.NumIn()
	if rv.Type().IsVariadic() {
		if numIn == 1 {
			switch len(args) {
			case 0:
				in = append(in, reflect.Zero(rt.In(0)))
			case 1:
				if args[0] == nil {
					in = append(in, reflect.Zero(rt.In(0)))
				} else {
					rvv := reflect.ValueOf(args[0])
					rtt := rvv.Type()
					if rtt.Kind() == reflect.Ptr {
						rtt = rtt.Elem()
					}
					if rtt.Kind() == reflect.Slice {
						in = append(in, rvv)
					} else {
						sliceVal := reflect.MakeSlice(rt.In(0), 1, 1)
						sliceVal.Index(0).Set(rvv)
						in = append(in, sliceVal)
					}
				}
			default:
				sliceVal := reflect.MakeSlice(rt.In(0), len(args), len(args))
				for i := 0; i < len(args); i++ {
					sliceVal.Index(i).Set(reflect.ValueOf(args[i]))
				}
				in = append(in, sliceVal)
			}
		} else if numIn > 1 {
			if len(args) == numIn {
				for i := 0; i < numIn-1; i++ {
					in = append(in, reflect.ValueOf(args[i]))
				}
				if args[numIn-1] == nil {
					in = append(in, reflect.Zero(rt.In(numIn-1)))
				} else {
					rvv := reflect.ValueOf(args[numIn-1])
					rtt := rvv.Type()
					if rtt.Kind() == reflect.Ptr {
						rtt = rtt.Elem()
					}
					if rtt.Kind() == reflect.Slice {
						in = append(in, rvv)
					} else {
						sliceVal := reflect.MakeSlice(rt.In(numIn-1), 1, 1)
						sliceVal.Index(0).Set(rvv)
						in = append(in, sliceVal)
					}
				}
			} else if len(args) > numIn {
				n := numIn - 1
				for i := 0; i < n; i++ {
					in = append(in, reflect.ValueOf(args[i]))
				}
				sliceVal := reflect.MakeSlice(rt.In(n), len(args)-n, len(args)-n)
				for i := n; i < len(args); i++ {
					sliceVal.Index(i - n).Set(reflect.ValueOf(args[i]))
				}
				in = append(in, sliceVal)
			} else if len(args) < numIn {
				if len(args) < numIn-1 {
					return nil, errors.New("too few input arguments")
				}
				for i := 0; i < len(args); i++ {
					in = append(in, reflect.ValueOf(args[i]))
				}
				in = append(in, reflect.Zero(rt.In(numIn-1)))
			}
		}
		out = rv.CallSlice(in)
	} else {
		if len(args) != numIn {
			return nil, errors.New("too few input arguments")
		}
		for i := 0; i < len(args); i++ {
			if args[i] != nil {
				in = append(in, reflect.ValueOf(args[i]))
				continue
			}
			in = append(in, reflect.Zero(rt.In(i)))
		}
		out = rv.Call(in)
	}
	var result = make([]any, len(out))
	for i := 0; i < len(out); i++ {
		result[i] = out[i].Interface()
	}
	return result, nil
}
