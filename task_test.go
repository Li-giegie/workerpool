package workerpool

import (
	"fmt"
	"testing"
)

func TestCall(t *testing.T) {
	call(func() { fmt.Println("func 1") })

	call(func(a int) {
		fmt.Println("func 2", a)
	}, 1)

	call(func(a int, b string) {
		fmt.Println("func 3", a, b)
	}, 1, "1")

	call(func(a int, b string) {
		fmt.Println("func 4", a, b)
	}, 1, "1")

	call(func(a []int) {
		fmt.Println("func 5", a)
	}, []int{1, 2})

	call(func(a []int) {
		fmt.Println("func 6", a)
	}, nil)

	call(func(a ...int) {
		fmt.Println("func 7", a)
	})

	call(func(a ...int) {
		fmt.Println("func 8", a)
	}, nil)

	call(func(a ...int) {
		fmt.Println("func 9", a)
	}, []int{1})

	call(func(a ...int) {
		fmt.Println("func 10", a)
	}, []int{1, 2, 3})

	call(func(a ...int) {
		fmt.Println("func 11", a)
	}, 1)

	call(func(a ...int) {
		fmt.Println("func 12", a)
	}, 1, 2, 3)

	call(func(a int, b ...int) {
		fmt.Println("func 13", a, b)
	}, 1)

	call(func(a int, b ...int) {
		fmt.Println("func 14", a, b)
	}, 1, nil)

	call(func(a int, b ...int) {
		fmt.Println("func 15", a, b)
	}, 1, []int{})

	call(func(a int, b ...int) {
		fmt.Println("func 16", a, b)
	}, 1, 1)

	call(func(a int, b ...int) {
		fmt.Println("func 17", a, b)
	}, 1, 1, 2, 3, 4, 5)

	call(func(a int, b ...int) {
		fmt.Println("func 18", a, b)
	}, 1, []int{1, 2, 3, 4, 5})
}

func BenchmarkCall(b *testing.B) {
	for i := 0; i < b.N; i++ {
		call(func(a int, b ...int) {
			//fmt.Println("func 18", a, b)
		}, 1, []int{1, 2, 3, 4, 5})
	}
}
