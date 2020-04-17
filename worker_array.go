package ants

import (
	"errors"
	"time"
)

var (
	errQueueIsFull = errors.New("the queue is full")
	errQueueIsReleased = errors.New("the queue length is zero")
)

//------------------------------------  workerArray接口  -----------------------------
//@todo v1版本对workers的存储就是 workers []*Worker,显得不够儒雅，这里我们通过定义统一的接口workerArray来对其进行进一步封装
//workers是一个slice，用来存放空闲worker，请求进入Pool之后会首先检查workers中是否有空闲worker，若有则取出绑定任务执行，
//否则判断当前运行的worker是否已经达到容量上限，是—阻塞等待，否—新开一个worker执行任务
type workerArray interface {
	len() int
	isEmpty() bool
	insert(worker *goWorker) error
	detach() *goWorker
	retrieveExpiry(duration time.Duration) []*goWorker
	reset()
}

//--------------- 创建workerArray的类型---------------------------
type arrayType int

const (
	stackType arrayType = 1 << iota
	loopQueueType
)

func newWorkerArray(aType arrayType, size int) workerArray {
	switch aType {
	case stackType://
		return newWorkerStack(size)
	case loopQueueType://
		return newWorkerLoopQueue(size)
	default:
		return newWorkerStack(size)
	}
}
