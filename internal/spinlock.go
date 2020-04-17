package internal

import (
	"runtime"
	"sync"
	"sync/atomic"
)
//@link https://www.jianshu.com/p/f3a6b514ce74
//自定义一个自旋锁
//@todo 不可重入的自旋锁
type spinLock uint32

func (sl *spinLock) Lock() {
	//值置为1，且让出当前G
	for !atomic.CompareAndSwapUint32((*uint32)(sl), 0, 1) {
		//用于让出CPU时间片，让出当前goroutine的执行权限，
		//调度器安排其它等待的任务运行，并在下次某个时候从该位置恢复执行。
		//这就像跑接力赛，A跑了一会碰到代码runtime.Gosched()就把接力棒交给B了，A歇着了，B继续跑。
		runtime.Gosched()
	}
}

func (sl *spinLock) Unlock() {
	atomic.StoreUint32((*uint32)(sl), 0)
}

//创建一个自旋锁
func NewSpinLock() sync.Locker {
	return new(spinLock)
}
