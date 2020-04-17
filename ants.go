package ants

import (
	"errors"
	"log"
	"math"
	"os"
	"runtime"
	"time"
)
//---------------------------------------------------------------------------

const (
	DefaultAntsPoolSize = math.MaxInt32 //默认池子大小
	DefaultCleanIntervalTime = time.Second //worker的默认过期时间
)
//---------------------------------------------------------------------------

const (
	OPENED = iota // OPENED represents that the pool is opened.
	CLOSED // CLOSED represents that the pool is closed.
)
//---------------------------------------------------------------------------

var (
	// Ants包报错提醒
	ErrInvalidPoolSize = errors.New("invalid size for pool")
	ErrLackPoolFunc = errors.New("must provide function for pool")
	ErrInvalidPoolExpiry = errors.New("invalid expiry for pool")
	ErrPoolClosed = errors.New("this pool has been closed")
	ErrPoolOverload = errors.New("too many goroutines blocked on submit or Nonblocking is set")
	//确定worker的通道是否该是缓冲通道，灵感来自fasthttp 主要取决于P的数量，P为1则...大于1则...
	workerChanCap = func() int {
		if runtime.GOMAXPROCS(0) == 1 {
			return 0
		}
		return 1
	}()
	//当导入包的时候就初始化一个默认的日志实例
	defaultLogger = Logger(log.New(os.Stderr, "", log.LstdFlags))
	//当导入包的时候就初始化一个默认的goroutine池子
	//defaultAntsPool, _ = NewPool(DefaultAntsPoolSize)
	//defaultAntsPool, _ = NewPool(DefaultAntsPoolSize,WithPreAlloc(true),WithExpiryDuration(1))
	//defaultAntsPool, _ = NewPool(DefaultAntsPoolSize,WithOptions(Options{1,false,0,false,nil,nil}))
)

//------------日志接口---------------------------------------------------------------
type Logger interface {
	Printf(format string, args ...interface{})
}


//----------------------为默认协程池提供的对外使用的几个公共方法---------------------------------

//提交具体的任务回调方法到默认池子中
func Submit(task func()) error {
	return defaultAntsPool.Submit(task)
}
//返回当前默认池子运行的goroutines的数量
func Running() int {
	return defaultAntsPool.Running()
}
//返回此默认池的容量
func Cap() int {
	return defaultAntsPool.Cap()
}
//返回可以继续创建的worker数量
func Free() int {
	return defaultAntsPool.Free()
}
//关闭默认的goroutine池子
func Release() {
	defaultAntsPool.Release()
}

// Reboot reboots the default pool.
func Reboot() {
	defaultAntsPool.Reboot()
}
