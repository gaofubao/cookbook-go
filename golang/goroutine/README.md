# Goroutine

## 底层原理
---
goroutine 可以理解为一种 Go 语言的协程（轻量级线程），是 Go 支持高并发的基础，属于用户态的线程，由 Go runtime 管理而不是操作系统。

### 底层数据结构
```go
// runtime/runtime2.go
type g struct {
    goid    int64   // 唯一的goroutine的ID
    sched   gobuf   // goroutine切换时，用于保存g的上下文
    stack   stack   // 栈
    gopc            // pc of go statement that created this goroutine
    startpc uintptr // pc of goroutine function
    // ...
}

type gobuf struct {
    sp   uintptr    // 栈指针位置
    pc   uintptr    // 运行到的程序位置
    g    guintptr   // 指向 goroutine
    ret  uintptr    // 保存系统调用的返回值
    // ...
}

type stack struct {
    lo uintptr  // 栈的下界内存地址
    hi uintptr  // 栈的上界内存地址
}
```
最终有一个 runtime.g 对象放入调度队列。

### 状态流转
![状态流转](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206031853992.jpg)

| 字段        | 状态       | 说明                                                         |
| ----------- | ---------- | ------------------------------------------------------------ |
| _Gidle      | 空闲中     | G 刚刚新建，仍未初始化                                       |
| _Grunnable  | 待运行     | 就绪状态，G 在运行队列中，等待 M 取出并运行                  |
| _Grunning   | 运行中     | M 正在运行这个 G，这时候 M 会拥有一个 P                      |
| _Gsyscall   | 系统调用中 | M 正在运行这个 G 发起的系统调用，这时候 M 并不拥有 P         |
| _Gwaiting   | 等待中     | G 在等待某些条件完成，这时 G 不在运行也不在运行队列中（可能在 channel 中） |
| _Gdead      | 已中止     | G 未被使用，可能已执行完毕                                   |
| _Gcopystack | 栈复制中   | G 正在获取一个新的栈空间并把原来的内容复制过去（用于防止 GC 扫描） |

#### 创建
通过 go 关键字调用底层函数 runtime.newproc() 创建一个 goroutine。
当调用该函数之后，goroutine 会被设置成 runnable 状态。
创建好的这个 goroutine 会新建一个自己的栈空间，同时在 G 的 sched 中维护栈地址与程序计数器这些信息。
每个 G 在被创建之后，都会被优先放入到本地队列中，如果本地队列已经满了，就会被放入到全局队列中。

#### 运行
goroutine 本身只是一个数据结构，真正让 goroutine 运行起来的是调度器。Go 实现了一个用户态的调度器（GMP 模型），这个调度器充分利用现代计算机的多核特性，同时让多个 goroutine 运行，同时 goroutine 设计的很轻量级，调度和上下文切换的代价都比较小。

![](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206031902973.png)

调度时机：
- 新起一个协程和协程执行完毕
- 会阻塞的系统调用，比如文件 io、网络 io
- channel、mutex 等阻塞操作
- time.sleep
- 垃圾回收之后
- 主动调用 runtime.Gosched()
- 运行过久或系统调用过久等等

每个 M 开始执行 P 本地队列中的 G 时，goroutine 会被设置成 running 状态。
- 如果某个 M 把本地队列中的 G 都执行完成之后，然后就会去全局队列中拿 G，这里需要注意，每次去全局队列拿 G 时，都需要上锁，避免同样的任务被多次拿。
- 如果全局队列都被拿完了，而当前 M 也没有更多的 G 可以执行时，它就会去其他 P 的本地队列中拿任务，这个机制被称之为 work stealing 机制，每次会拿走一半的任务，向下取整，比如另一个 P 中有 3 个任务，那一半就是一个任务。
- 当全局队列为空，M 也没办法从其他的 P 中拿任务时，就会让自身进入自选状态，等待有新的 G 进来。最多只会有 GOMAXPROCS 个 M 在自旋状态，过多 M 的自旋会浪费 CPU 资源。

#### 阻塞
channel 的读写操作、等待锁、等待网络数据、系统调用等都有可能发生阻塞，会调用底层函数 runtime.gopark()，会让出 CPU 时间片，让调度器安排其它等待的任务运行，并在下次某个时候从该位置恢复执行。
当调用该函数之后，goroutine 会被设置成 waiting 状态。

#### 唤醒
处于 waiting 状态的 goroutine，在调用 runtime.goready() 函数之后会被唤醒，唤醒的 goroutine 会被重新放到 M 对应的上下文 P 对应的 runqueue 中，等待被调度。
当调用该函数之后，goroutine 会被设置成 runnable 状态。

#### 退出
当 goroutine 执行完成后，会调用底层函数 runtime.Goexit()。
当调用该函数之后，goroutine 会被设置成 dead 状态。

## goroutine 和线程的区别
---

|            | goroutine                                                    | 线程                                                         |
| ---------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| 内存占用   | 创建一个 goroutine 的栈内存消耗为 2KB，实际运行过程中，如果栈空间不够用，会自动进行扩容。 | 创建一个线程的栈内存消耗为 1MB。                            |
| 创建和销毀 | Goroutine 因为是由 Go runtime 负责管理的，创建和销毁的消耗非常小，是用户级。 | 线程创建和销毀都会有巨大的消耗，因为要和操作系统打交道，是内核级的，通常解决的办法就是线程池。 |
| 切换       | goroutines 切换只需保存三个寄存器：PC、SP、BP。goroutine 的切换约为 200ns，相当于 2400-3600 条指令。 | 当线程切换时，需要保存各种寄存器，以便恢复现场。线程切换会消耗 1000-1500ns，相当于 12000-18000 条指令。 |

## goroutine 泄露场景
---
### 泄露原因
- goroutine 内进行 channel/mutex 等读写操作被一直阻塞。
- goroutine 内的业务逻辑进入死循环，资源一直无法释放。
- goroutine 内的业务逻辑进入长时间等待，有不断新增的 goroutine 进入等待。

### 泄露场景
如果输出的 goroutines 数量是在不断增加的，就说明存在泄漏。

#### nil channel
channel 如果忘记初始化，那么无论你是读，还是写操作，都会造成阻塞。
```go
func block() {
	var ch chan int
	for i := 0; i < 10; i++ {
		go func() {
			<-ch
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}
```
输出结果：
```shell
# go run main.go
before goroutines:  1
after goroutines:  11
```

#### channel 发送不接收
channel 发送数量超过 channel 接收数量，就会造成阻塞。
```go
func block() {
	ch := make(chan int)
	for i := 0; i < 10; i++ {
		go func() {
			ch <- 1
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}
```
输出结果：
```shell
# go run main.go
before goroutines:  1
after goroutines:  11
```

#### channel 接收不发送
channel 接收数量超过 channel 发送数量，也会造成阻塞。
```go
func block() {
	ch := make(chan int)
	for i := 0; i < 10; i++ {
		go func() {
			<-ch
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}
```
输出结果：
```shell
# go run main.go
before goroutines:  1
after goroutines:  11
```

#### http request body 未关闭
resp.Body.Close() 未被调用时，goroutine 不会退出。
```go
var wg sync.WaitGroup

// http request body 未关闭
func requestWithNoClose() {
	resp, err := http.Get("https://www.baidu.com")
	if err != nil {
		fmt.Printf("error occurred while fetching page, error code: %d, error: %s", resp.StatusCode, err.Error())
	}
	// defer resp.Body.Close()
}

func block() {
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			requestWithNoClose()
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
	wg.Wait()
}
```
输出结果：
```shell
# go run main.go
before goroutines:  1
after goroutines:  21
```

#### 互斥锁忘记解锁
第一个协程获取 sync.Mutex 锁，但是它可能在处理业务逻辑，又或是忘记 Unlock 了。因此导致后面的协程想加锁，却因锁未释放被阻塞了。
```go
func block() {
	var mutex sync.Mutex
	for i := 0; i < 10; i++ {
		go func() {
			mutex.Lock()
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}
```
输出结果：
```shell
# go run main.go
before goroutines:  1
after goroutines:  10
```

#### sync.WaitGroup 使用不当
由于 wg.Add 的数量与 wg.Done 数量并不匹配，因此在调用 wg.Wait 方法后一直阻塞等待。
```go
func block() {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		go func() {
			wg.Add(2)
			wg.Done()
			wg.Wait()
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}
```
输出结果：
```shell
# go run main.go
before goroutines:  1
after goroutines:  11
```

### 如何排查
- 单个函数：调用 runtime.NumGoroutine() 方法来打印执行代码前后 goroutine 的运行数量，进行前后比较，就能知道有没有泄露了。
- 生产/测试环境：使用 pprof 实时监测 goroutine 的数量。

## 查看正在执行 goroutine 数量
---
### 程序中引入 pprof pakage
通过 `import _ "net/http/pprof"` 引入 pprof 包。
```go
func main() {
	for i := 0; i < 100; i++ {
		go func() {
			select {}
		}()
	}

	_ = http.ListenAndServe("localhost:6060", nil)
}
```
执行程序：
```shell
# go run main.go
```
查看 profile：
1. 通过浏览器访问
    - http://localhost:6060/debug/pprof
2. 通过交互终端访问
    - go tool pprof 'http://localhost:6060/debug/pprof/profile'
3. 查看可视化界面
    - wget http://localhost:6060/debug/pprof/profile
    - go tool pprof -http=:6001 profile

Profile 说明：
- allocs: A sampling of all past memory allocations
- block: Stack traces that led to blocking on synchronization primitives
- cmdline: The command line invocation of the current program
- goroutine: Stack traces of all current goroutines
- heap: A sampling of memory allocations of live objects. You can specify the gc GET parameter to run GC before taking the heap sample.
- mutex: Stack traces of holders of contended mutexes
- profile: CPU profile. You can specify the duration in the seconds GET parameter. After you get the profile file, use the go tool pprof command to investigate the profile.
- threadcreate: Stack traces that led to the creation of new OS threads
- trace: A trace of execution of the current program. You can specify the duration in the seconds GET parameter. After you get the trace file, use the go tool trace command to investigate the trace.

### 分析 goroutine 文件
执行以下命令：
```shell
go tool pprof -http=:6001 http://localhost:6060/debug/pprof/goroutine
```
![pprof](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041345157.png)

## 控制并发 goroutine 数量
---
在开发过程中，如果不对 goroutine 加以控制而进行滥用的话，可能会导致服务整体崩溃。比如耗尽系统资源导致程序崩溃，或者 CPU 使用率过高导致系统忙不过来。

通常使用 channel 控制 goroutine 并发的数量：

### 有缓冲 channel
利用缓冲满时发送阻塞的特性。
```go
var wg sync.WaitGroup

func Read(ch chan bool, i int) {
	fmt.Printf("goroutine_num: %d, go func: %d\n", runtime.NumGoroutine(), i)
	<-ch
	wg.Done()
}

func main() {
	// 模拟用户请求数量
	requestCount := 10
	fmt.Println("goroutine_num", runtime.NumGoroutine())
	// 管道长度即最大并发数
	ch := make(chan bool, 3)
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		ch <- true
		go Read(ch, i)
	}

	wg.Wait()
}
```
输出结果：默认最多不超过 3（4-1）个 goroutine 并发执行
```shell
# go run main.go 
goroutine_num 1
goroutine_num: 4, go func: 2
goroutine_num: 4, go func: 3
goroutine_num: 4, go func: 4
goroutine_num: 4, go func: 5
goroutine_num: 4, go func: 6
goroutine_num: 4, go func: 7
goroutine_num: 4, go func: 8
goroutine_num: 4, go func: 9
goroutine_num: 3, go func: 1
goroutine_num: 4, go func: 0
```

### 无缓冲 channel
任务发送和执行分离，指定消费者并发协程数。
```go
var wg sync.WaitGroup

func Read(ch chan bool, i int) {
	for range ch {
		fmt.Printf("goroutine_num: %d, go func: %d\n", runtime.NumGoroutine(), i)
		wg.Done()
	}
}

func main() {
	// 模拟用户请求数量
	requestCount := 10
	fmt.Println("goroutine_num", runtime.NumGoroutine())
	ch := make(chan bool)
	for i := 0; i < 3; i++ {
		go Read(ch, i)
	}

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		ch <- true
	}

	wg.Wait()
}
```
输出结果：
```shell
# go run main.go 
goroutine_num 1
goroutine_num: 4, go func: 2
goroutine_num: 4, go func: 2
goroutine_num: 4, go func: 2
goroutine_num: 4, go func: 2
goroutine_num: 4, go func: 0
goroutine_num: 4, go func: 0
goroutine_num: 4, go func: 1
goroutine_num: 4, go func: 2
goroutine_num: 4, go func: 1
goroutine_num: 4, go func: 0
```
