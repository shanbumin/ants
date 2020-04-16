

package main

import (
	"fmt"
	"github.com/panjf2000/ants/v2"
	"sync"
	"time"
)

func demoFunc() {
	time.Sleep(1 * time.Second)
	fmt.Println("Hello World!")
}

//创建池子的时候不指明回调
//@todo pool, _ := ants.NewPool()
//@todo defer pool.Release()
//@todo pool.Invoke()
func main() {
	//初始化变量
	runTimes := 1000
	var wg sync.WaitGroup
	//创建一个池子(未指明任何回到方法)
	pool, _ := ants.NewPool(10)
	defer pool.Release()

	//任务回调函数
	syncCalculateSum := func() {
		demoFunc()
		wg.Done()
	}
	//循环提交任务
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		pool.Submit(syncCalculateSum)
	}
	//等待
	wg.Wait()
	fmt.Printf("running goroutines: %d\n", pool.Running())
	fmt.Printf("finish all tasks.\n")
}
