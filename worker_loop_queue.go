package ants
import "time"

//实现workerArray接口,6个方法额
//@todo 这里是用切片实现了一个队列机制吧了,先进先出额
//@todo [head] w1 w2 w3 w4 w5 ... [tail]  ===>每次添加都是从tail处，每次获取都是从head处
type loopQueue struct {
	items  []*goWorker
	expiry []*goWorker
	head   int  //标记最后一个取出元素的下标参考值
	tail   int  //标记最后一个元素的下标参考值
	size   int //队列长度，一般是与池子中的size值一致
	isFull bool //队列是否已满
}


//获取队列中元素的个数
//@reviser sam@2020-04-18 14:29:24
func (wq *loopQueue) len() int {
	if wq.size == 0 {
		return 0
	}
	if wq.head == wq.tail {
		if wq.isFull {
			return wq.size
		}
		return 0
	}
	if wq.tail > wq.head {
		return wq.tail - wq.head
	}

	return wq.size - wq.head + wq.tail
}

//判断队列是否已经空了
//@reviser sam@2020-04-18 11:11:31
func (wq *loopQueue) isEmpty() bool {
	return wq.head == wq.tail && !wq.isFull //头尾一样，且还未满，则证明就是空的啊
}

//往队列的尾部添加一个worker
//@reviser sam@2020-04-18 10:55:37
func (wq *loopQueue) insert(worker *goWorker) error {
	//当前队列的缓存长度是否为0
	if wq.size == 0 {
		return errQueueIsReleased
	}
    //队列是否已满
	if wq.isFull {
		return errQueueIsFull
	}
	//追加到尾部
	wq.items[wq.tail] = worker
	wq.tail++
	//如果此时添加之后正好等于size则tail置为0
	if wq.tail == wq.size {
		wq.tail = 0
	}
	//上述在已经添满情况下，head此刻还等于0，则队列是满的状态，还没有从头取
	if wq.tail == wq.head {
		wq.isFull = true
	}
	return nil
}
//从队列的头部取出一个元素
//@reviser sam@2020-04-18 11:09:16
func (wq *loopQueue) detach() *goWorker {
	//判断队列是否已经空了
	if wq.isEmpty() {
		return nil
	}
	//从头部取
	w := wq.items[wq.head]
	wq.head++
	//如果此时正好取到了最后一个了，则重置head为0，方便下次继续从头再取
	if wq.head == wq.size {
		wq.head = 0
	}
	//取出之后，肯定队列处于非满状态
	wq.isFull = false

	return w
}

func (wq *loopQueue) retrieveExpiry(duration time.Duration) []*goWorker {
	if wq.isEmpty() {
		return nil
	}

	wq.expiry = wq.expiry[:0]
	expiryTime := time.Now().Add(-duration)

	for !wq.isEmpty() {
		if expiryTime.Before(wq.items[wq.head].recycleTime) {
			break
		}
		wq.expiry = append(wq.expiry, wq.items[wq.head])
		wq.head++
		if wq.head == wq.size {
			wq.head = 0
		}
		wq.isFull = false
	}

	return wq.expiry
}

//恢复出厂设置
//@reviser sam@2020-04-18 14:31:02
func (wq *loopQueue) reset() {
	if wq.isEmpty() {
		return
	}

Releasing:
	if w := wq.detach(); w != nil {
		w.task <- nil
		goto Releasing
	}
	wq.items = wq.items[:0]
	wq.size = 0
	wq.head = 0
	wq.tail = 0
}
//---------------------------------------------------
//创建workers容器队列,size是cap
//cap 是按照 1  2  4   8  16  32   64 ...增长的
//@reviser sam@2020-04-18 10:21:41
func newWorkerLoopQueue(size int) *loopQueue {
	return &loopQueue{
		items: make([]*goWorker, size),
		size:  size,
	}
}