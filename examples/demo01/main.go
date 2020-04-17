package main

import (
	"fmt"
	"sync"
	"time"
	"github.com/panjf2000/ants/v2"
)

func demoFunc() {
	time.Sleep(1 * time.Second)
	fmt.Println("Hello World!")
}

//todo 未指明创建池子，则使用默认的池子,这种提交的任务只支持提交任务的时候指明回调函数
//todo  ants.Release()
//todo  ants.Submit(syncCalculateSum)
func main() {
	defer ants.Release()
	//自定义变量
	var wg sync.WaitGroup
	//任务回调函数
	syncCalculateSum := func() {
		demoFunc()
		wg.Done()
	}
	//循环提交任务
	runTimes := 1000
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		_ = ants.Submit(syncCalculateSum) //提交整个任务回调到Pool
	}
	//等待...
	wg.Wait()
	fmt.Printf("running goroutines: %d\n", ants.Running())
	fmt.Printf("finish all tasks.\n")
}

