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
	state int32 //该池子是否已经关闭了,1表示关闭了,todo v1版本是用字段release表示的额
	lock sync.Locker //lock是一个互斥锁/读写锁的接口类型，用以支持Pool的同步操作,v1版本这里是 sync.Mutex
	cond *sync.Cond //该条件变量是为了等待获取一个空闲的worker
	workerCache sync.Pool 	//原子操作之临时对象池workerCache加速了函数retrieveWorker中可用worker的获取。
	blockingNum int 	//当前已经处于阻塞中的任务个数(即都在等待空闲worker的到来)
	options *Options
}

//定期清理池子中过期的worker
//@reviser sam@2020-04-17 14:45:25
func (p *Pool) periodicallyPurge() {
	//开启一个连续定时器,既然worker的过期时间是expiryDuration,那定时器就每隔expiryDuration进行清理是再好不过了
	heartbeat := time.NewTicker(p.options.ExpiryDuration)
	defer heartbeat.Stop()
	//定期循环
	for range heartbeat.C {
		//Load 只保证读取的不是正在写入的值
		if atomic.LoadInt32(&p.state) == CLOSED {
			break
		}
        //(1)清理过期workers,以前是未封装成方法的，赤裸裸的遍历所有的workers，然后比对过期时间进行删除的,现在不光封装成方法了，而且采用了二分查找的方式
		p.lock.Lock()
		expiredWorkers := p.workers.retrieveExpiry(p.options.ExpiryDuration)
		p.lock.Unlock()
		//(2)通知过时的worker停止。
		//该通知必须在p.lock之外，因为w.task可能会阻塞并且可能会花费大量时间,如果许多workers位于非本地CPU上.
		//@todo 所以v1版本在这里的处理也是放到lock中的，所以是非常不理智的
		for i := range expiredWorkers {
			expiredWorkers[i].task <- nil
		}
		//(3)当该池子中没有正在执行任务的worker了，则可以尝试唤醒那些还卡在p.cond.Wait()的程序了
		if p.Running() == 0 {
			p.cond.Broadcast()
		}
	}
}




//提交任务到池子中，真牛逼，任务是提交给该池子中的某个worker的task通道上的，每个worker都有自己的专属通道
//@todo v1版是无脑式的提交，v2版更加灵活一下，可以设置提交的阻塞数量的
//@reviser sam@2020-04-17 16:29:20
func (p *Pool) Submit(task func()) error {
	//判断pool是否关闭了  当p.state被设置为1即表示释放了，即已经关闭了
	if atomic.LoadInt32(&p.state) == CLOSED {
		return ErrPoolClosed
	}
	//获取一个可用worker之后，将task添加到worker的task字段中
	//这里可以看成开辟了一个任务通道，且是该任务通道的生产端
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


// incRunning increases the number of the currently running goroutines.
func (p *Pool) incRunning() {
	atomic.AddInt32(&p.running, 1)
}

// decRunning decreases the number of the currently running goroutines.
func (p *Pool) decRunning() {
	atomic.AddInt32(&p.running, -1)
}

//从池子中返回一个可用的worker用来执行任务
//1.优先先从worker.items中获取空闲的worker
//2.如果未超过池子限制，则从临时对象池中获取即可(没有会按照New字段创建新的worker),总之从临时对象池中获取的worker都是需要重新run的
//@return 返回w证明是成功的，返回nil，则证明是too many goroutines blocked on submit or Nonblocking is set true
//@link https://www.cnblogs.com/yang-2018/p/11133580.html todo 条件变量的巧妙运用
//@reviser sam@2020-04-17 16:49:15
func (p *Pool) retrieveWorker() *goWorker {
	//初始化变量
	var w *goWorker
	spawnWorker := func() { //从临时对象池中获取"新"worker
		w = p.workerCache.Get().(*goWorker) //返回的是接口类型，需要类型断言一下
		w.run()
	}
	//准备操作workers这个切片了，所以一定要上锁，防止并发问题
	p.lock.Lock()

	w = p.workers.detach()
	if w != nil { //a.取出来那就解锁就好了，直接会结束if分支，进入return w的
		p.lock.Unlock()
	} else if p.Running() < p.Cap() { //b.当前无空闲worker但是池子还没有超过限制
		p.lock.Unlock()
		spawnWorker()
	} else { //c.池子容量已满，新请求等待还是直接打回头，看具体参数设置
		//c1.任务如果是非阻塞的,则返回nil,即不可以继续再添加了，如果
		if p.options.Nonblocking {
			p.lock.Unlock()
			return nil
		}
		//如果是阻塞的，即非非阻塞的，则不停的循环获取一个空闲worker(前提是未超过 MaxBlockingTasks)
	Reentry:
		//-------------------------
		//判断提交的任务是否已经超过阻塞限制的个数了
		if p.options.MaxBlockingTasks != 0 && p.blockingNum >= p.options.MaxBlockingTasks {
			p.lock.Unlock()
			return nil
		}
		p.blockingNum++
		p.cond.Wait() //这里的内涵很深额
		p.blockingNum--
		//条件变量收到通知之后，p.Running()是有可能为0的额
		if p.Running() == 0 {
			p.lock.Unlock()
			spawnWorker()
			return w
		}
        //继续从items中获取一个空闲的
		w = p.workers.detach()
		if w == nil {
			goto Reentry
		}
		//-------------------------
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
//@reviser sam@2020-04-17 11:00:57
func NewPool(size int, options ...Option) (*Pool, error) {
	//(1)配置参数的加载、检测以及默认值设置
	//检测池子的大小
	if size <= 0 {
		return nil, ErrInvalidPoolSize
	}
    //将每个参数项的值设置集中到opts中
	opts := loadOptions(options...)
    //空闲worker的过期时间，未设置，默认是1s
	if expiry := opts.ExpiryDuration; expiry < 0 {
		return nil, ErrInvalidPoolExpiry
	} else if expiry == 0 {
		opts.ExpiryDuration = DefaultCleanIntervalTime
	}
    //日志处理驱动的设置
	if opts.Logger == nil {
		opts.Logger = defaultLogger
	}
	//(2)创建池子实例
	p := &Pool{
		capacity: int32(size),
		lock:     internal.NewSpinLock(),
		options:  opts,
	}
	//(3)池子需要动态配置的几个属性字段
	//设置临时对象池用来创建新对象值的模板，就是创建一个goWorker实例
	p.workerCache.New = func() interface{} {
		return &goWorker{
			pool: p,
			task: make(chan func(), workerChanCap),//make(chan func(),1)
		}
	}
	//在初始化Pool时是否对内存进行预分配
	if p.options.PreAlloc {
		p.workers = newWorkerArray(loopQueueType, size)
	} else {
		p.workers = newWorkerArray(stackType, 0)
	}
	//初始化条件变量
	p.cond = sync.NewCond(p.lock)
	//(4)专门启动一个定时任务以及启动定期清理过期worker任务，独立goroutine运行
	go p.periodicallyPurge()

	return p, nil
}