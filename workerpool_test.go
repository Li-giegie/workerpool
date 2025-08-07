package workerpool

import (
	"fmt"
	"log"
	"sync"
	"testing"
	"time"
)

func TestFuncBase(t *testing.T) {
	wp := NewWorkerPool(1, 12)
	go func() {
		time.Sleep(time.Second)
		wp.Push(func() {
			fmt.Println("Activate task")
			go wp.Stop()
		})
	}()
	err := wp.Run()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestFuncIn(t *testing.T) {
	wp := NewWorkerPool(1, 12)
	wp.AddListener(EventEndTask, func(e Event, a ...any) {
		task := a[0].(Task)
		log.Println("start worker id", task.Id(), "err", task.Err(), "out", task.Out())
	})
	go func() {
		time.Sleep(time.Second)
		wp.Push(func(a, b int) int {
			fmt.Println("Activate task")
			go wp.Stop()
			return a + b
		}, 1, 2)
	}()
	err := wp.Run()
	if err != nil {
		t.Error(err)
		return
	}
}

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
