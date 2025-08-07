# workerpool 一个灵活高效的 Go 任务调度池

workerpool 一个灵活高效的 Go 任务调度池

## 什么是 workerpool？
workerpool 是一个轻量级的 Go 任务调度池实现，它允许你：
- 控制并发执行的工作协程数量
- 灵活处理不同签名的函数和参数
- 通过事件机制监控任务生命周期
- 管理任务队列和等待队列，避免资源耗尽
## 快速开始
使用 go get 命令快速安装：
```go
go get -u  github.com/Li-giegie/workerpool
```

## 基础用法
```go
package main

import (
	"fmt"
	"time"
	"github.com/Li-giegie/workerpool"
)

func main() {
	// 创建工作池：1个工作协程，任务队列容量12
	wp := workerpool.NewWorkerPool(1, 12)

	// 异步推送任务
	go func() {
		time.Sleep(time.Second)
		wp.Push(func() {
			fmt.Println("任务执行中...")
			// 任务完成后停止工作池
			go wp.Stop()
		})
	}()

	// 启动工作池（阻塞）
	if err := wp.Run(); err != nil {
		fmt.Println("工作池启动失败:", err)
	}
}
```

## 核心功能详解

### 1. 支持任意函数签名和参数
workerpool 最强大的特性之一是支持任意函数签名和参数，无需提前定义特定类型：

```go
// 带参数和返回值的任务示例
func main() {
    wp := workerpool.NewWorkerPool(1, 12)
    
    // 添加任务完成事件监听
    wp.AddListener(workerpool.EventEndTask, func(e workerpool.Event, a ...any) {
        task := a[0].(workerpool.Task)
        fmt.Printf("任务ID: %v, 错误: %v, 结果: %v\n", 
            task.Id(), task.Err(), task.Out())
    })
    
    go func() {
        time.Sleep(time.Second)
        // 推送带参数的函数
        wp.Push(func(a, b int) int {
            fmt.Println("执行加法任务")
            go wp.Stop()
            return a + b
        }, 10, 20) // 传递参数 10 和 20
    }()
    
    wp.Run()
}
```
### 2. 灵活的队列配置
创建工作池时可以配置三个关键参数：
- 创建工作池时可以配置三个关键参数：
- 任务队列容量
- 等待队列容量
```go
// 示例：2个工作协程，任务队列容量50，等待队列容量100
wp := workerpool.NewWorkerPool(2, 50, 100)
```
队列机制说明：
- 新任务优先放入任务队列（缓冲区）
- 任务队列满时，放入等待队列
- 等待队列也满时，返回 ErrOverflow 错误
- 工作协程空闲时会自动从等待队列获取任务

### 3. 事件监听机制
workerpool 提供了完整的事件机制，让你可以监控整个任务生命周期：
```go
// 支持的事件类型
const (
    EventPushTask    // 任务被推送时
    EventStartTask   // 任务开始执行时
    EventEndTask     // 任务执行结束时
    EventStartWorker // 工作协程启动时
)

// 示例：添加多个事件监听
wp.AddListener(workerpool.EventStartTask, func(e workerpool.Event, a ...any) {
    task := a[0].(workerpool.Task)
    fmt.Printf("任务 %v 开始执行\n", task.Id())
})

wp.AddListener(workerpool.EventEndTask, func(e workerpool.Event, a ...any) {
    task := a[0].(workerpool.Task)
    fmt.Printf("任务 %v 执行结束\n", task.Id())
}, true) // 最后一个参数为true表示异步执行回调
```
### 4. 工作池管理
workerpool 提供了丰富的管理方法：
```go
// 重启工作池
err := wp.Reboot()

// 停止工作池
wp.Stop()

// 获取待执行任务数量
waitNum := wp.WaitTaskNum()

// 获取可用任务队列容量
free := wp.Free()

// 获取当前工作池状态
state := wp.State()
```

## 实现原理简析
### 核心数据结构
workerpool 的核心是 workerPool 结构体，主要包含：
```go
type workerPool struct {
    numWorker    int           // 工作协程数量
    numTaskQueue int           // 任务队列容量
    numWaitQueue int           // 等待队列容量
    lock         sync.Mutex    // 同步锁
    wg           sync.WaitGroup // 用于等待所有工作协程完成
    taskQueue    chan Task     // 任务队列（缓冲区）
    waitQueue    []Task        // 等待队列
    eventMap     sync.Map      // 事件处理器映射
    state        WorkerPoolState // 工作池状态
}
```
### 任务执行流程

1. 调用 Push 方法添加任务时，会先尝试放入 taskQueue
2. 若 taskQueue 已满，则放入 waitQueue（如果有容量）
3. 工作协程从 taskQueue 中获取任务并执行
4. 任务执行完成后，工作协程会检查 waitQueue，并将任务移至 taskQueue

### 反射调用机制
workerpool 能支持任意函数签名，核心在于使用了 Go 的反射机制：
```go
// call 函数通过反射动态调用任意函数
func call(fn any, args ...any) ([]any, error) {
    rv := reflect.ValueOf(fn)
    // ... 反射处理逻辑 ...
    out = rv.Call(in) // 或 rv.CallSlice(in) 处理可变参数
    // ... 结果转换 ...
}
```

## 性能考量
为了验证 workerpool 的性能，我们可以对比直接使用 goroutine 和使用工作池的性能差异：
```go
// 工作池性能测试
func BenchmarkWorkerPool(b *testing.B) {
    wp := NewWorkerPool(12, 12)
    go wp.Run()
    time.Sleep(time.Millisecond * 100)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        err := wp.Push(func() {
            _ = i
        })
        if err != nil {
            b.Error(err)
            return
        }
    }
}

// 直接使用goroutine的性能测试
func BenchmarkGoroutine(b *testing.B) {
    var wg sync.WaitGroup
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _ = i
        }()
    }
    wg.Wait()
}
```
在大量短任务场景下，workerpool 可以避免频繁创建和销毁 goroutine 的开销，从而提升性能。

## 总结
workerpool 是一个简单而强大的任务调度池，它的主要优势包括：
- 接口简洁易用，学习成本低
- 支持任意函数签名和参数，灵活性高
- 完善的事件机制，便于监控和扩展
- 合理的队列管理，避免资源耗尽
- 良好的性能表现，适合处理大量并发任务

如果你正在寻找一个轻量级的 Go 任务调度解决方案，workerpool 值得一试。它的源码简洁明了，也适合作为学习 Go 并发编程的参考示例。