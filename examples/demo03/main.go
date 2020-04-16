

package main

import (
	"fmt"
	"github.com/panjf2000/ants/v2"
	"sync"
	"sync/atomic"
)

var sum int32

func myFunc(i interface{}) {
	//将interface类型转成int32
	n := i.(int32)
	//追加到sum变量中
	atomic.AddInt32(&sum, n)
	fmt.Printf("run with %d\n", n)
}

//创建池子的时候指明回调
//@todo pool, _ := ants.NewPoolWithFunc()
//@todo defer pool.Release()
//@todo pool.Invoke()
func main() {
	//初始化变量
	runTimes := 1000
	var wg sync.WaitGroup
	//一个执行批量同类任务的协程池，PoolWithFunc相较于Pool，因为一个池只绑定一个任务函数，
	//省去了每一次task都需要传送一个任务函数的代价，因此其性能优势比起Pool更明显
	pool, _ := ants.NewPoolWithFunc(10, func(i interface{}) {
		myFunc(i)
		wg.Done()
	})
	defer pool.Release()
	//循环提交任务
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		_ = pool.Invoke(int32(i)) //所有任务回调一样，所以创建池子的时候就指明了，这里只需要指明回调参数即可
	}
	//等待
	wg.Wait()
	fmt.Printf("running goroutines: %d\n", pool.Running())
	fmt.Printf("finish all tasks, result is %d\n", sum)
	if sum != 499500 {
		panic("the final result is wrong!!!")
	}
}
