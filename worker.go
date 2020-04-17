package ants

import (
	"runtime"
	"time"
)


// goWorker是执行任务的实际执行者， 它启动一个接受任务的goroutine并执行函数调用。
//@todo  task就是一个通道，里面放置了该worker要执行的任务额
//@todo  v1版是 type Worker struct {}

type goWorker struct {
	pool *Pool //拥有该worker的池子指针
	task chan func() //任务回调函数
	recycleTime time.Time //将worker重新放入队列时，recycleTime将被更新。
}
//运行启动goroutine以重复该过程,执行函数调用。
func (w *goWorker) run() {
	w.pool.incRunning()
	go func() {
		defer func() {
			w.pool.decRunning()
			w.pool.workerCache.Put(w)
			if p := recover(); p != nil {
				if ph := w.pool.options.PanicHandler; ph != nil {
					ph(p)
				} else {
					w.pool.options.Logger.Printf("worker exits from a panic: %v\n", p)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					w.pool.options.Logger.Printf("worker exits from panic: %s\n", string(buf[:n]))
				}
			}
		}()

		for f := range w.task {
			if f == nil {
				return
			}
			f()
			if ok := w.pool.revertWorker(w); !ok {
				return
			}
		}
	}()
}
