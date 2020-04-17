package ants

import "time"

//通过函数类型来表示选项配置更具灵活性
//一个Option类型表示一个选项配置
//@reviser sam@2020-04-17 10:32:21
type Option func(opts *Options)

//加载每个Option统一返回出去
//@reviser sam@2020-04-17 10:32:38
func loadOptions(options ...Option) *Options {
	opts := new(Options)
	for _, option := range options {
		//option是个方法，传递opts指针可以对其进行设置额
		option(opts)
	}
	return opts
}

//----------------------Options结构体包含了初始化一个ants池时需要的所有参数选项-----------------------------------------
type Options struct {
	ExpiryDuration time.Duration //是worker的过期时长，在空闲队列中的worker的最新一次运行时间与当前时间之差如果大于这个值则表示已过期，定时清理任务会清理掉这个worker 单位秒
	PreAlloc bool 	//在初始化Pool时是否对内存进行预分配。
	MaxBlockingTasks int //pool.Submit上的goroutine阻止的最大任务数量，0表示没有限制
	Nonblocking bool //任务提交是否是不闭塞的
	PanicHandler func(interface{}) //自定义的处理每个worker中发生的panic函数
	Logger Logger //自定义日志驱动
}

//创建goroutine池的时候指明所有的参数配置
//defaultAntsPool, _ = NewPool(DefaultAntsPoolSize,WithOptions(Options{1,false,0,false,nil,nil}))
//@reviser sam@2020-04-17 10:54:49
func WithOptions(options Options) Option {
	return func(opts *Options) {
		*opts = options
	}
}

//-----------------将参数配置项转成函数式-----------------------------------------------------------------------------------

//设置worker的清理间隔时间
//@reviser sam@2020-04-17 10:42:04
func WithExpiryDuration(expiryDuration time.Duration) Option {
	return func(opts *Options) {
		opts.ExpiryDuration = expiryDuration
	}
}
//在初始化Pool时是否对内存进行预分配。
//@reviser sam@2020-04-17 10:41:54
func WithPreAlloc(preAlloc bool) Option {
	return func(opts *Options) {
		opts.PreAlloc = preAlloc
	}
}
//pool.Submit上的goroutine阻止的最大任务数量，0表示没有限制
//@reviser sam@2020-04-17 10:41:40
func WithMaxBlockingTasks(maxBlockingTasks int) Option {
	return func(opts *Options) {
		opts.MaxBlockingTasks = maxBlockingTasks
	}
}

//当Nonblocking为true时，Pool.Submit将永远不会被阻塞。
//当无法一次完成Pool.Submit时，将返回ErrPoolOverload。
//当Nonblocking为true时，MaxBlockingTasks不起作用。
//@reviser sam@2020-04-17 10:43:56
func WithNonblocking(nonblocking bool) Option {
	return func(opts *Options) {
		opts.Nonblocking = nonblocking
	}
}
//自定义panic处理函数
//@reviser sam@2020-04-17 10:42:59
func WithPanicHandler(panicHandler func(interface{})) Option {
	return func(opts *Options) {
		opts.PanicHandler = panicHandler
	}
}
// 自定义logger处理驱动
// @reviser sam@2020-04-17 10:43:38
func WithLogger(logger Logger) Option {
	return func(opts *Options) {
		opts.Logger = logger
	}
}
