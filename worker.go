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
//@reviser sam@2020-04-18 09:07:26
func (w *goWorker) run() {
	//增加当前运行的worker的数量
	w.pool.incRunning()
	//开启一个G执行worker要处理的任务
	go func() {
		//捕获一些错误
		defer func() {
			//@todo 只要该函数结束，不管错不错都会执行这两句
			w.pool.decRunning() //正在运行的w个数减一
			w.pool.workerCache.Put(w) //worker开启groutine之后，出现恐慌的，则会被放入临时对象池中额
			//-------
			if p := recover(); p != nil {
				//有自定义的按照自定义的处理
				if ph := w.pool.options.PanicHandler; ph != nil {
					ph(p)
				} else {
					//默认的异常处理方式
					w.pool.options.Logger.Printf("worker exits from a panic: %v\n", p)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					w.pool.options.Logger.Printf("worker exits from panic: %s\n", string(buf[:n]))
				}
			}
		}()
		// 循环监听取出的w的任务通道，一旦有任务立马取出运行
		for f := range w.task {
			//提交的任务回调是nil，则没有必要开启一个新的worker
			if f == nil {
				return
			}
			f()
			//执行完任务就将worker放入items中
			if ok := w.pool.revertWorker(w); !ok {
				return
			}
		}
	}()
}
