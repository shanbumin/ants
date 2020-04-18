package ants

import (
	"time"
)

//实现workerArray接口,6个方法额
type workerStack struct {
	items  []*goWorker
	expiry []*goWorker
	size   int
}

//获取items的个数
func (wq *workerStack) len() int {
	return len(wq.items)
}
//判断items是否为空
func (wq *workerStack) isEmpty() bool {
	return len(wq.items) == 0
}
//往items中追加一个元素
func (wq *workerStack) insert(worker *goWorker) error {
	wq.items = append(wq.items, worker)
	return nil
}
//从items的尾部取出一个元素
//当前有可用worker，从队列尾部取出一个使用，正好实现LIFO，后进先出  栈
//后进(最后进来的那个索引是最大的额)的证明是最近活跃的worker，所以优先它们出去
//w1 w2  w3  w4  ... w10 w11  w12 ...因为越靠前就是越接近过期,所以从尾部取
//@todo v1版本有bug,直接就从workers中获取,有可能尾部的那个就是失效的额
func (wq *workerStack) detach() *goWorker {
	l := wq.len()
	if l == 0 {
		return nil
	}

	w := wq.items[l-1]
	wq.items = wq.items[:l-1]

	return w
}

//遍历items找出所有的过期的worker放入expiry中
//因为采用了LIFO后进先出  栈的结构存放空闲worker，所以该workers默认已经是按照worker的最后运行时间由远及近排序，
//w1 w2  w3  w4  ... w10 w11  w12 ...   如果判断出w10过期了，那么w1-9必然也过期了
//@reviser sam@2020-04-17 14:54:30
func (wq *workerStack) retrieveExpiry(duration time.Duration) []*goWorker {
	n := wq.len()
	if n == 0 {
		return nil
	}
    //小于这个时间的worker都是过期的
	expiryTime := time.Now().Add(-duration)
	//对items的所有元素进行二分查找，找到那个临界的index值
	index := wq.binarySearch(0, n-1, expiryTime)

	wq.expiry = wq.expiry[:0]//先清空所有
	if index != -1 {
		wq.expiry = append(wq.expiry, wq.items[:index+1]...) //如果index是过期的，则index左边的全部都是过期的
		m := copy(wq.items, wq.items[index+1:])
		wq.items = wq.items[:m]
	}
	return wq.expiry
}
//二分查找,给个左右区间的序号,比如0-100,假设index为49就是我们要的值，则mid的变化为：
//[0,100] ===>50
//[0,49] ===>24
//[25,49] ===>37
//[38,49] ===>43
//[44,49] ===>46
//[47,49] ===>48
//[49,49] ====>49
//[50,49]
//@todo 目的是快速找到过期的临界元素
//@reviser sam@2020-04-17 15:09:33
func (wq *workerStack) binarySearch(l, r int, expiryTime time.Time) int {
	var mid int
	//查找结束的条件是最右index不再大于最左边的index了。
	for l <= r {
		mid = (l + r) / 2 //这里由于mid是int类型，所以会自己舍弃小数的
		if expiryTime.Before(wq.items[mid].recycleTime) { //中间的未过期===>向左查找
			r = mid - 1
		} else {   //中间的已经过期了,左边肯定已经全过期了，所以想进一步靠近临界值，l得变成mid+1 ===>向右查找
			l = mid + 1
		}
	}
	return r
}

//恢复出厂设置
//@reviser sam@2020-04-18 10:00:54
func (wq *workerStack) reset() {
	for i := 0; i < wq.len(); i++ {
		wq.items[i].task <- nil
	}
	wq.items = wq.items[:0]
}
//-----------------------------------------------------------------------
//创建workers容器栈,size是cap
//cap 是按照 1  2  4   8  16  32   64 ...增长的
//@reviser sam@2020-04-17 14:27:43
func newWorkerStack(size int) *workerStack {
	return &workerStack{
		items: make([]*goWorker, 0, size),
		size:  size,
	}
}