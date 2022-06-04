# 并发编程

## 常用并发模型
---
并发模型说的是系统中的线程如何协作完成并发任务，不同的并发模型，线程以不同的方式进行通信和协作。

### 线程间通信方式
线程间通信方式有两种：共享内存和消息传递，无论是哪种通信模型，线程或者协程最终都会从内存中获取数据，所以更为准确的说法是直接共享内存、发送消息的方式来同步信息。

|          | 共享内存                                                     | 发送消息                                                     |
| -------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| 抽象层级 | 抽象层级低，当我们遇到对资源进行更细粒度的控制或者对性能有极高要求的场景才应该考虑抽象层级更低的方法。 | 抽象层级高，提供了更良好的封装和与领域更相关和契合的设计，比如 Go 语言中的 channel 就提供了 goroutine 之间用于传递信息的方式，它在内部实现时就广泛用到了共享内存和锁，通过对两者进行的组合提供了更高级的同步机制。 |
| 耦合度   | 高，线程需要在读取或者写入数据时先获取保护该资源的互斥锁。   | 低，生产消费者模型。                                         |
| 线程竞争 | 需要加锁，才能避免线程竞争和数据冲突。                       | 保证同一时间只有一个活跃的线程能够访问数据，channel 维护所有被该 channel 阻塞的协程，保证有资源时只唤醒一个协程，从而避免竞争。 |

Go 语言中实现了两种并发模型，一种是共享内存并发模型，另一种则是 CSP 模型。

### 共享内存并发模型
通过直接共享内存 + 锁的方式同步信息，多用于传统多线程并发。

![共享内存](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041406652.png)

### CSP 并发模型
通过发送消息的方式来同步信息，Go 语言推荐使用的通信顺序进程（Communicating Sequential Processes）并发模型，通过 goroutine 和 channel 来实现。

![CSP](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041407001.png)

- goroutine 是 Go 语言中并发的执行单位，可以理解为”线程“。
- channel 是 Go 语言中各个并发结构体（goroutine）之前的通信机制。 通俗的讲，就是各个 goroutine 之间通信的“管道“，类似于 Linux 中的管道。

## 并发同步原语
---
Go 是一门以并发编程见长的语言，它提供了一系列的同步原语以方便开发者使用。

### 原子操作
Mutex、RWMutex 等并发原语的底层实现是通过 atomic 包中的一些原子操作来实现的，原子操作是最基础的并发原语。

![atomic](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041409511.jpeg)

```go
var opts int64 = 0

// atomic
func main() {
	add(&opts, 3)
	store(&opts, 6)
	load(&opts)
	compareAndSwap(&opts, 3, 4)
	swap(&opts, 5)
}

func add(addr *int64, delta int64) {
	atomic.AddInt64(addr, delta)
	fmt.Println("add opts: ", *addr)
}

func store(addr *int64, newValue int64) {
	atomic.StoreInt64(addr, newValue)
	fmt.Println("store opts: ", *addr)
}

func load(addr *int64) {
	fmt.Println("load opts: ", atomic.LoadInt64(&opts))
}

func compareAndSwap(addr *int64, oldValue int64, newValue int64) {
	if atomic.CompareAndSwapInt64(addr, oldValue, newValue) {
		fmt.Println("cas opts: ", *addr)
		return
	}
}
```

### Channel
channel 管道是高级同步原语，goroutine 之间通信的桥梁。
使用场景：消息队列、数据传递、信号通知、任务编排、锁

```go
func main() {
	ch := make(chan struct{}, 1)
	for i := 0; i < 10; i++ {
		go func() {
			ch <- struct{}{}
			time.Sleep(1 * time.Second)
			fmt.Println("通过channel访问临界区")
			<-ch
		}()
	}

	select {}
}
```

### 基本并发原语
Go 语言在 sync 包中提供了用于同步的一些基本原语，这些基本原语提供了较为基础的同步功能，但是它们是一种相对原始的同步机制，在多数情况下，我们都应该使用抽象层级更高的 channel 实现同步。
常见的并发原语如下：sync.Mutex、sync.RWMutex、sync.WaitGroup、sync.Cond、sync.Once、sync.Pool、sync.Context

#### sync.Mutex
sync.Mutex（互斥锁）可以限制对临界资源的访问，保证只有一个 goroutine 访问共享资源。
使用场景：大量读写，比如多个 goroutine 并发更新同一个资源，像计数器

```go
func main() {
	// 封装好的计数器
	var (
		counter Counter
		wg      sync.WaitGroup
		gNum    = 10
	)

	wg.Add(gNum)
	// 启动10个goroutine
	for i := 0; i < gNum; i++ {
		go func() {
			defer wg.Done()
			counter.Incr() // 受到锁保护的方法
		}()
	}
	wg.Wait()
	fmt.Println(counter.Count())
}

// 线程安全的计数器类型
type Counter struct {
	mu    sync.Mutex
	count uint64
}

// 加1的方法，内部使用互斥锁保护
func (c *Counter) Incr() {
	c.mu.Lock()
	c.count++
	c.mu.Unlock()
}

// 得到计数器的值，也需要锁保护
func (c *Counter) Count() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}
```

#### sync.RWMutex
sync.RWMutex（读写锁）可以限制对临界资源的访问，保证只有一个 goroutine 写共享资源，可以有多个 goroutine 读共享资源。
使用场景：大量并发读，少量并发写，有强烈的性能要求

```go
func main() {
	// 封装好的计数器
	var (
		counter Counter
		gNum    = 10
	)

	// 启动10个goroutine
	for i := 0; i < gNum; i++ {
		go func() {
			counter.Count() // 受到锁保护的方法
			fmt.Println(counter.Count())
		}()
	}

	for { // 一个writer
		counter.Incr() // 计数器写操作
		fmt.Println("incr")
		time.Sleep(time.Second)
	}
}

// 线程安全的计数器类型
type Counter struct {
	mu    sync.RWMutex
	count uint64
}

// 加1的方法，内部使用互斥锁保护
func (c *Counter) Incr() {
	c.mu.Lock()
	c.count++
	c.mu.Unlock()
}

// 得到计数器的值，也需要锁保护
func (c *Counter) Count() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.count
}
```

#### sync.WaitGroup
sync.WaitGroup 可以等待一组 goroutine 的返回。
使用场景：并发等待，任务编排，一个比较常见的使用场景是批量发出 RPC 或者 HTTP 请求

```go
func main() {
	list := []string{"A", "B", "C", "D"}
	wg := &sync.WaitGroup{}
	wg.Add(len(list))

	for _, char := range list {
		go func(char string) {
			defer wg.Done()
			fmt.Println(char)
		}(char)
	}
	wg.Wait()
}
```

#### sync.Cond
sync.Cond 可以让一组的 goroutine 都在满足特定条件时被唤醒。
使用场景：利用等待/通知机制实现阻塞或者唤醒

```go
var status int64

// cond
func main() {
	c := sync.NewCond(&sync.Mutex{})
	for i := 0; i < 10; i++ {
		go listen(c)
	}
	time.Sleep(1 * time.Second)

	go broadcast(c)
	time.Sleep(1 * time.Second)
}

func broadcast(c *sync.Cond) {
	c.L.Lock()
	atomic.StoreInt64(&status, 1)
	c.Signal()
	c.L.Unlock()
}

func listen(c *sync.Cond) {
	c.L.Lock()
	for atomic.LoadInt64(&status) != 1 {
		c.Wait()
	}
	fmt.Println("listen")
	c.L.Unlock()
}
```

#### sync.Once
sync.Once 可以保证在 Go 程序运行期间的某段代码只会执行一次。
使用场景：常常用于单例对象的初始化场景
```go
func main() {
	once := &sync.Once{}

	for i := 0; i < 10; i++ {
		once.Do(func() {
			fmt.Println("only once")
		})
	}
}
```

#### sync.Pool
sync.Pool 可以将暂时将不用的对象缓存起来，待下次需要的时候直接使用，不用再次经过内存分配，复用对象的内存，减轻 GC 的压力，提升系统的性能（频繁地分配、回收内存会给 GC 带来一定的负担，严重的时候会引起 CPU 的毛刺）。
使用场景：对象池化， TCP 连接池、数据库连接池、Worker Pool

```go
func main() {
	pool := sync.Pool{
		New: func() interface{} {
			return 0
		},
	}

	for i := 0; i < 10; i++ {
		v := pool.Get().(int)
		fmt.Println(v) // 取出来的值是put进去的，对象复用；如果是新建对象，则取出来的值为0
		pool.Put(i)
	}
}
```

#### sync.Map
sync.Map 线程安全的 map。
使用场景：map 并发读写

```go
func main() {
	var scene sync.Map
	// 将键值对保存到sync.Map
	scene.Store("1", 1)
	scene.Store("2", 2)
	scene.Store("3", 3)

	// 从sync.Map中根据键取值
	fmt.Println(scene.Load("1"))

	// 根据键删除对应的键值对
	scene.Delete("1")
	// 遍历所有sync.Map中的键值对

	scene.Range(func(k, v interface{}) bool {
		fmt.Println("iterate:", k, v)
		return true
	})
}
```

#### sync.Context
sync.Context 可以进行上下文信息传递、提供超时和取消机制、控制子 goroutine 的执行。
使用场景：取消一个 goroutine 的执行

```go
func main() {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer func() {
			fmt.Println("goroutine exit")
		}()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("receive cancel signal!")
				return
			default:
				fmt.Println("default")
				time.Sleep(time.Second)
			}
		}
	}()

	time.Sleep(time.Second)
	cancel()
	time.Sleep(2 * time.Second)
}
```

### 扩展并发原语

#### ErrGroup
errgroup 可以在一组 goroutine 中提供了同步、错误传播以及上下文取消的功能。
使用场景：只要一个 goroutine 出错我们就不再等其他 goroutine 了，减少资源浪费，并且返回错误

```go
func main() {
	var g errgroup.Group
	var urls = []string{
		"http://www.baidu.com/",
		"https://www.sina.com.cn/",
	}

	for i := range urls {
		url := urls[i]
		g.Go(func() error {
			resp, err := http.Get(url)
			if err == nil {
				resp.Body.Close()
			}
			return err
		})
	}

	if err := g.Wait(); err == nil {
		fmt.Println("Successfully fetched all URLs.")
	} else {
		fmt.Println("fetched error:", err.Error())
	}
}
```

#### Semaphore
Semaphore 带权重的信号量，控制多个 goroutine 同时访问资源。
使用场景：控制 goroutine 的阻塞和唤醒

```go
func main() {
	ctx := context.Background()

	for i := range task {
		// 如果没有worker可用，会阻塞在这里，直到某个worker被释放
		if err := sema.Acquire(ctx, 1); err != nil {
			break
		}
		// 启动worker goroutine
		go func(i int) {
			defer sema.Release(1)
			time.Sleep(100 * time.Millisecond) // 模拟一个耗时操作
			task[i] = i + 1
		}(i)
	}

	// 请求所有的worker,这样能确保前面的worker都执行完
	if err := sema.Acquire(ctx, int64(maxWorkers)); err != nil {
		log.Printf("获取所有的worker失败: %v", err)
	}
	fmt.Println(maxWorkers, task)
}
```

#### SingleFlight
用于抑制对下游的重复请求。
使用场景：访问缓存、数据库等场景，缓存过期时只有一个请求去更新数据库

```go
var count int32

func main() {
	time.AfterFunc(1*time.Second, func() {
		atomic.AddInt32(&count, -count)
	})

	var (
		wg  sync.WaitGroup
		now = time.Now()
		n   = 1000
		sg  = &singleflight.Group{}
	)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			res, _ := singleflightGetArticle(sg, 1)
			// res, _ := getArticle(1)
			if res != "article: 1" {
				panic("err")
			}
			wg.Done()
		}()
	}

	wg.Wait()
	fmt.Printf("同时发起 %d 次请求，耗时: %s", n, time.Since(now))
}

// 模拟从数据库读取
func getArticle(id int) (article string, err error) {
	// 假设这里会对数据库进行调用, 模拟不同并发下耗时不同
	atomic.AddInt32(&count, 1)
	time.Sleep(time.Duration(count) * time.Millisecond)

	return fmt.Sprintf("article: %d", id), nil
}

// 模拟优先读缓存，缓存不存在读取数据库，并且只有一个请求读取数据库，其它请求等待
func singleflightGetArticle(sg *singleflight.Group, id int) (string, error) {
	v, err, _ := sg.Do(fmt.Sprintf("%d", id), func() (interface{}, error) {
		return getArticle(id)
	})

	return v.(string), err
}
```

## WaitGroup 实现原理
---
Go 标准库提供了 WaitGroup 原语, 可以用它来等待一批 goroutine 结束。

### 底层数据结构
```go
// A WaitGroup must not be copied after first use.
type WaitGroup struct {
    noCopy noCopy
    state1 [3]uint32
}
```

- noCopy 是 Go 源码中检测禁止拷贝的技术。如果程序中有 WaitGroup 的赋值行为，使用 go vet 检查程序时，就会发现有报错。但需要注意的是，noCopy 不会影响程序正常的编译和运行。
- state1 主要是存储着状态和信号量，状态维护了 2 个计数器，一个是请求计数器 counter ，另外一个是等待计数器 waiter（已调用 WaitGroup.Wait 的 goroutine 的个数）。

当数组的首地址是处于一个 8 字节对齐的位置上时，那么就将这个数组的前 8 个字节作为 64 位值使用表示状态，后 4 个字节作为 32 位值表示信号量(semaphore)；同理如果首地址没有处于 8 字节对齐的位置上时，那么就将前 4 个字节作为 semaphore，后 8 个字节作为 64 位数值。

### 使用方法
在 WaitGroup 里主要有3个方法：
- WaitGroup.Add()：可以添加或减少请求的 goroutine 数量，Add(n) 将会导致 counter += n
- WaitGroup.Done()：相当于 Add(-1)，Done() 将导致 counter -=1，请求计数器 counter 为 0 时通过信号量调用 runtime_Semrelease 唤醒 waiter 线程
- WaitGroup.Wait()：会将 waiter++，同时通过信号量调用 runtime_Semacquire(semap) 阻塞当前 goroutine

```go
func main() {
    var wg sync.WaitGroup

    for i := 1; i <= 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            println("hello")
        }()
    }

    wg.Wait()
}
```

## Cond 实现原理
---
Go 标准库提供了 Cond 原语，可以让 goroutine 在满足特定条件时被阻塞和唤醒。

### 底层数据结构
```go
type Cond struct {
    noCopy noCopy

    // L is held while observing or changing the condition
    L Locker

    notify  notifyList
    checker copyChecker
}

type notifyList struct {
    wait   uint32
    notify uint32
    lock   uintptr // key field of the mutex
    head   unsafe.Pointer
    tail   unsafe.Pointer
}
```

- nocopy：Go 源码中检测禁止拷贝的技术。如果程序中有 WaitGroup 的赋值行为，使用 go vet 检查程序时，就会发现有报错，但需要注意的是，noCopy 不会影响程序正常的编译和运行。
- checker：用于禁止运行期间发生拷贝，双重检查（Double check）。
- L：可以传入一个读写锁或互斥锁，当修改条件或者调用 Wait 方法时需要加锁。
- notify：通知链表，调用 Wait() 方法的 goroutine 会放到这个链表中，从这里获取需被唤醒的 goroutine 列表。

### 使用方法
在 Cond 里主要有3个方法：
- sync.NewCond(l Locker): 新建一个 sync.Cond 变量，注意该函数需要一个 Locker 作为必填参数，这是因为在 cond.Wait() 中底层会涉及到 Locker 的锁操作
- Cond.Wait(): 阻塞等待被唤醒，调用 Wait 函数前需要先加锁；并且由于 Wait 函数被唤醒时存在虚假唤醒等情况，导致唤醒后发现，条件依旧不成立，因此需要使用 for 语句来循环地进行等待，直到条件成立为止
- Cond.Signal(): 只唤醒一个最先 Wait 的 goroutine，可以不用加锁
- Cond.Broadcast(): 唤醒所有 Wait 的 goroutine，可以不用加锁

```go
var status int64

func main() {
    c := sync.NewCond(&sync.Mutex{})
    for i := 0; i < 10; i++ {
        go listen(c)
    }
    go broadcast(c)
    time.Sleep(1 * time.Second)
}

func broadcast(c *sync.Cond) {
    // 原子操作
    atomic.StoreInt64(&status, 1) 
    c.Broadcast()
}

func listen(c *sync.Cond) {
    c.L.Lock()
    for atomic.LoadInt64(&status) != 1 {
        c.Wait() 
        // Wait 内部会先调用 c.L.Unlock()，来先释放锁，如果调用方不先加锁的话，会报错
    }
    fmt.Println("listen")
    c.L.Unlock()
}
```

## 共享变量安全读写方式
---

| 方法                                                         | 并发原语                           | 备注                                                  |
| ------------------------------------------------------------ | ---------------------------------- | ----------------------------------------------------- |
| 不要修改变量                                                 | sync.Once                          | 不要去写变量，变量只初始化一次                        |
| 只允许一个 goroutine 访问变量                                | Channel                            | 不要通过共享变量来通信，通过通信（channel）来共享变量 |
| 允许多个 goroutine 访问变量，但是同一时间只允许一个 goroutine 访问 | sync.Mutex、sync.RWMutex、原子操作 | 实现锁机制，同时只有一个线程能拿到锁                  |


## 数据竞争问题
---
只要有两个以上的 goroutine 并发访问同一变量，且至少其中的一个是写操作的时候就会发生数据竞争；全是读的情况下是不存在数据竞争的。

```go
func main() {
	i := 0

	go func() {
		i++ // write i
	}()

	fmt.Println(i) // read i
}
```
使用 race 参数检测代码中的数据竞争：
```shell
# go run -race main.go
0
==================
WARNING: DATA RACE
Write at 0x00c0000160e8 by goroutine 7:
  main.main.func1()
      /Users/gaofubao/go/src/cookbook-go/golang/concurrency/race/main.go:9 +0x44

Previous read at 0x00c0000160e8 by main goroutine:
  main.main()
      /Users/gaofubao/go/src/cookbook-go/golang/concurrency/race/main.go:12 +0xba

Goroutine 7 (running) created at:
  main.main()
      /Users/gaofubao/go/src/cookbook-go/golang/concurrency/race/main.go:8 +0xb0
==================
Found 1 data race(s)
exit status 66
```
