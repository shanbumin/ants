package ants

import (
	"github.com/panjf2000/ants/v2/internal"
	"sync"
	"sync/atomic"
	"time"
)

//Pool接受来自客户端的任务，它通过回收goroutine将goroutine的总数限制为给定数目。
//Pool是一个通用的goroutine池，支持不同类型的任务，亦即每一个任务绑定一个函数提交到池中，批量执行不同类型任务，是一种广义的goroutine池；
//本项目中还实现了另一种goroutine池 — 批量执行同类任务的goroutine池PoolWithFunc (pool_func.go)，每一个PoolWithFunc只会绑定一个任务函数pf，
//这种Pool适用于大批量相同任务的场景，因为每个Pool只绑定一个任务函数，因此PoolWithFunc相较于Pool会更加节省内存，但通用性就不如前者了，
//为了让大家更好地理解goroutine池的原理，这里我们用通用的Pool来分析。
//@todo 一个Pool结构体吧了
type Pool struct {
	capacity int32 //是该Pool的容量，也就是开启worker数量的上限，每一个worker绑定一个goroutine
	running int32  //是当前正在执行任务的worker(goroutines)数量
	workers workerArray 	// workers is a slice that store the available workers.
	state int32 // state is used to notice the pool to closed itself.
	lock sync.Locker //lock是一个互斥锁/读写锁的接口类型，用以支持Pool的同步操作,v1版本这里是 sync.Mutex
	cond *sync.Cond //该条件变量是为了等待获取一个空闲的worker
	workerCache sync.Pool 	//原子操作之临时对象池workerCache加速了函数retrieveWorker中可用worker的获取。
	blockingNum int 	// blockingNum is the number of the goroutines already been blocked on pool.Submit, protected by pool.lock
	options *Options
}

// periodicallyPurge clears expired workers periodically.
func (p *Pool) periodicallyPurge() {
	heartbeat := time.NewTicker(p.options.ExpiryDuration)
	defer heartbeat.Stop()

	for range heartbeat.C {
		if atomic.LoadInt32(&p.state) == CLOSED {
			break
		}

		p.lock.Lock()
		expiredWorkers := p.workers.retrieveExpiry(p.options.ExpiryDuration)
		p.lock.Unlock()

		// Notify obsolete workers to stop.
		// This notification must be outside the p.lock, since w.task
		// may be blocking and may consume a lot of time if many workers
		// are located on non-local CPUs.
		for i := range expiredWorkers {
			expiredWorkers[i].task <- nil
		}

		// There might be a situation that all workers have been cleaned up(no any worker is running)
		// while some invokers still get stuck in "p.cond.Wait()",
		// then it ought to wakes all those invokers.
		if p.Running() == 0 {
			p.cond.Broadcast()
		}
	}
}



// ---------------------------------------------------------------------------

// Submit submits a task to this pool.
func (p *Pool) Submit(task func()) error {
	if atomic.LoadInt32(&p.state) == CLOSED {
		return ErrPoolClosed
	}
	var w *goWorker
	if w = p.retrieveWorker(); w == nil {
		return ErrPoolOverload
	}
	w.task <- task
	return nil
}

// Running returns the number of the currently running goroutines.
func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

// Free returns the available goroutines to work.
func (p *Pool) Free() int {
	return p.Cap() - p.Running()
}

// Cap returns the capacity of this pool.
func (p *Pool) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

// Tune changes the capacity of this pool.
func (p *Pool) Tune(size int) {
	if size < 0 || p.Cap() == size || p.options.PreAlloc {
		return
	}
	atomic.StoreInt32(&p.capacity, int32(size))
}

// Release Closes this pool.
func (p *Pool) Release() {
	atomic.StoreInt32(&p.state, CLOSED)
	p.lock.Lock()
	p.workers.reset()
	p.lock.Unlock()
}

// Reboot reboots a released pool.
func (p *Pool) Reboot() {
	if atomic.CompareAndSwapInt32(&p.state, CLOSED, OPENED) {
		go p.periodicallyPurge()
	}
}

// ---------------------------------------------------------------------------

// incRunning increases the number of the currently running goroutines.
func (p *Pool) incRunning() {
	atomic.AddInt32(&p.running, 1)
}

// decRunning decreases the number of the currently running goroutines.
func (p *Pool) decRunning() {
	atomic.AddInt32(&p.running, -1)
}

// retrieveWorker returns a available worker to run the tasks.
func (p *Pool) retrieveWorker() *goWorker {
	var w *goWorker
	spawnWorker := func() {
		w = p.workerCache.Get().(*goWorker)
		w.run()
	}

	p.lock.Lock()

	w = p.workers.detach()
	if w != nil {
		p.lock.Unlock()
	} else if p.Running() < p.Cap() {
		p.lock.Unlock()
		spawnWorker()
	} else {
		if p.options.Nonblocking {
			p.lock.Unlock()
			return nil
		}
	Reentry:
		if p.options.MaxBlockingTasks != 0 && p.blockingNum >= p.options.MaxBlockingTasks {
			p.lock.Unlock()
			return nil
		}
		p.blockingNum++
		p.cond.Wait()
		p.blockingNum--
		if p.Running() == 0 {
			p.lock.Unlock()
			spawnWorker()
			return w
		}

		w = p.workers.detach()
		if w == nil {
			goto Reentry
		}

		p.lock.Unlock()
	}
	return w
}

// revertWorker puts a worker back into free pool, recycling the goroutines.
func (p *Pool) revertWorker(worker *goWorker) bool {
	if atomic.LoadInt32(&p.state) == CLOSED || p.Running() > p.Cap() {
		return false
	}
	worker.recycleTime = time.Now()
	p.lock.Lock()

	err := p.workers.insert(worker)
	if err != nil {
		p.lock.Unlock()
		return false
	}

	// Notify the invoker stuck in 'retrieveWorker()' of there is an available worker in the worker queue.
	p.cond.Signal()
	p.lock.Unlock()
	return true
}

// ---------------------------------------------------------------------------

//@todo 创建一个goroutine池(未指明统一的任务处理方法额)
func NewPool(size int, options ...Option) (*Pool, error) {
	//检测池子的大小
	if size <= 0 {
		return nil, ErrInvalidPoolSize
	}
    //将每个参数项的值设置集中到opts中
	opts := loadOptions(options...)

	if expiry := opts.ExpiryDuration; expiry < 0 {
		return nil, ErrInvalidPoolExpiry
	} else if expiry == 0 {
		opts.ExpiryDuration = DefaultCleanIntervalTime
	}

	if opts.Logger == nil {
		opts.Logger = defaultLogger
	}

	p := &Pool{
		capacity: int32(size),
		lock:     internal.NewSpinLock(),
		options:  opts,
	}
	p.workerCache.New = func() interface{} {
		return &goWorker{
			pool: p,
			task: make(chan func(), workerChanCap),
		}
	}
	if p.options.PreAlloc {
		p.workers = newWorkerArray(loopQueueType, size)
	} else {
		p.workers = newWorkerArray(stackType, 0)
	}

	p.cond = sync.NewCond(p.lock)

	// Start a goroutine to clean up expired workers periodically.
	go p.periodicallyPurge()

	return p, nil
}