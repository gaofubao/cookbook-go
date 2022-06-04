# Channel

## channel 实现原理
---

### 概述

Go 中的 channel 是一个队列，遵循先进先出的原则，负责协程之间的通信（Go 语言提倡不要通过共享内存来通信，而要通过通信来实现内存共享，CSP（Communicating Sequential Process）并发模型，就是通过 goroutine 和 channel 来实现的）。

### 使用场景
- 停止信号监听
- 定时任务
- 生产方和消费方解耦
- 控制并发数

### 底层数据结构
通过 var 声明或 make 函数创建的 channel 变量是一个存储在函数栈帧上的指针，占用 8 个字节，指向堆上的 hchan 结构体。
源码包中 src/runtime/chan.go 定义了 hchan 的数据结构：
```go
type hchan struct {
    closed   uint32   // channel是否关闭的标志
    elemtype *_type   // channel中的元素类型

    // channel分为无缓冲和有缓冲两种。
    // 对于有缓冲的channel存储数据，使用了 ring buffer（环形缓冲区) 来缓存写入的数据，本质是循环数组
    // 为啥是循环数组？普通数组不行吗，普通数组容量固定更适合指定的空间，弹出元素时，普通数组需要全部都前移
    // 当下标超过数组容量后会回到第一个位置，所以需要有两个字段记录当前读和写的下标位置
    buf      unsafe.Pointer // 指向底层循环数组的指针（环形缓冲区）
    qcount   uint           // 循环数组中的元素数量
    dataqsiz uint           // 循环数组的长度
    elemsize uint16         // 元素的大小
    sendx    uint           // 下一次写下标的位置
    recvx    uint           // 下一次读下标的位置

    // 尝试读取channel或向channel写入数据而被阻塞的goroutine
    recvq    waitq  // 读等待队列
    sendq    waitq  // 写等待队列

    lock mutex //互斥锁，保证读写channel时不存在并发竞争问题
}
```

等待队列为双向链表，包含一个头结点和一个尾结点。
每个节点是一个 sudog 结构体变量，记录哪个协程在等待，等待的是哪个 channel，等待发送/接收的数据在哪里。
```go
type waitq struct {
    first *sudog
    last  *sudog
}

type sudog struct {
    g       *g
    next    *sudog
    prev    *sudog
    elem    unsafe.Pointer 
    c       *hchan 
    // ...
}
```

![hchan](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206042000855.png)

### 操作

#### 创建
使用 make(chan T, cap) 来创建 channel，make 语法会在编译时，转换为 makechan64 和 makechan。
```go
func makechan64(t *chantype, size int64) *hchan {
    if int64(int(size)) != size {
        panic(plainError("makechan: size out of range"))
    }

    return makechan(t, int(size))
}
```
创建 channel 有两种，一种是带缓冲的 channel，一种是不带缓冲的 channel：
```go
// 带缓冲
ch := make(chan int, 3)
// 不带缓冲
ch := make(chan int)
```

创建时会做一些检查:
- 元素大小不能超过 64K
- 元素的对齐大小不能超过 maxAlign 也就是 8 字节
- 计算出来的内存是否超过限制

创建时的策略:
- 如果是无缓冲的 channel，会直接给 hchan 分配内存
- 如果是有缓冲的 channel，并且元素不包含指针，那么会为 hchan 和底层数组分配一段连续的地址
- 如果是有缓冲的 channel，并且元素包含指针，那么会为 hchan 和底层数组分别分配地址

#### 发送
发送操作，编译时转换为 runtime.chansend 函数：
```go
func chansend(c *hchan, ep unsafe.Pointer, block bool, callerpc uintptr) bool 
```

阻塞式，调用 chansend 函数，并且 block=true：
```go
ch <- 10
```

非阻塞式，调用 chansend 函数，并且 block=false：
```go
select {
    case ch <- 10:
    // ...
    default
}
```

向 channel 中发送数据时大概分为两大块：检查和数据发送，数据发送流程如下：
- 如果 channel 的读等待队列存在接收者 goroutine
    - 将数据直接发送给第一个等待的 goroutine， 唤醒接收的 goroutine
- 如果 channel 的读等待队列不存在接收者 goroutine
    - 如果循环数组 buf 未满，那么将会把数据发送到循环数组 buf 的队尾
    - 如果循环数组 buf 已满，这个时候就会走阻塞发送的流程，将当前 goroutine 加入写等待队列，并挂起等待唤醒

#### 接收
发送操作，编译时转换为 runtime.chanrecv 函数：
```go
func chanrecv(c *hchan, ep unsafe.Pointer, block bool) (selected, received bool)
```

阻塞式，调用 chanrecv 函数，并且 block=true：
```go
<-ch
v := <-ch
v, ok := <-ch

// 当channel关闭时，for循环会自动退出，无需主动监测channel是否关闭，
// 可以防止读取已经关闭的channel,造成读到数据为通道所存储的数据类型的零值
for i := range ch {
    fmt.Println(i)
}
```
非阻塞式，调用 chanrecv 函数，并且 block=false：
```go
select {
    case <-ch:
    // ...
    default
}
```

向 channel 中接收数据时大概分为两大块，检查和数据发送，而数据接收流程如下：
- 如果 channel 的写等待队列存在发送者 goroutine
    - 如果是无缓冲 channel，直接从第一个发送者 goroutine 那里把数据拷贝给接收变量，唤醒发送的 goroutine
    - 如果是有缓冲 channel（已满），将循环数组 buf 的队首元素拷贝给接收变量，将第一个发送者 goroutine 的数据拷贝到 buf 循环数组队尾，唤醒发送的 goroutine
- 如果 channel 的写等待队列不存在发送者 goroutine
    - 如果循环数组 buf 非空，将循环数组 buf 的队首元素拷贝给接收变量
    - 如果循环数组 buf 为空，这个时候就会走阻塞接收的流程，将当前 goroutine 加入读等待队列，并挂起等待唤醒

#### 关闭
关闭操作，调用 close 函数，编译时转换为 runtime.closechan 函数。
```go
close(ch)
```

```go
func closechan(c *hchan)
```

#### 示例
```go
func main() {
	// ch是长度为4的带缓冲的channel
	// 初始hchan结构体重的buf为空，sendx和recvx均为0
	ch := make(chan string, 4)
	fmt.Println(ch, unsafe.Sizeof(ch))
	go sendTask(ch)
	go receiveTask(ch)
	time.Sleep(1 * time.Second)
}

// G1是发送者
// 当G1向ch里发送数据时，首先会对buf加锁，然后将task存储的数据copy到buf中，然后sendx++，然后释放对buf的锁
func sendTask(ch chan string) {
	taskList := []string{"this", "is", "a", "demo"}
	for _, task := range taskList {
		ch <- task //发送任务到channel
	}
}

// G2是接收者
// 当G2消费ch的时候，会首先对buf加锁，然后将buf中的数据copy到task变量对应的内存里，然后recvx++,并释放锁
func receiveTask(ch chan string) {
	for {
		task := <-ch                  //接收任务
		fmt.Println("received", task) //处理任务
	}
}
```

#### 总结
hchan 结构体的主要组成部分有四个：
- 用来保存 goroutine 之间传递数据的循环数组：buf
- 用来记录此循环数组当前发送或接收数据的下标值：sendx 和 recvx
- 用于保存向该 chan 发送和从该 chan 接收数据被阻塞的 goroutine 队列： sendq 和 recvq
- 保证 channel 写入和读取数据时线程安全的锁：lock

## 缓冲和非缓冲 channel
---
channel 有 2 种类型：无缓冲、有缓冲

|          | 无缓冲             | 有缓冲                |
| -------- | ------------------ | --------------------- |
| 创建方式 | make(chan TYPE)    | make(chan TYPE, SIZE) |
| 发送阻塞 | 数据接收前发送阻塞 | 缓冲满时发送阻塞      |
| 接收阻塞 | 数据发送前接收阻塞 | 缓冲空时接收阻塞      |

非缓冲 channel：
```go
func main() {
	ch := make(chan int)
	ch <- 1
	go loop(ch)
	time.Sleep(1 * time.Millisecond)
}

func loop(ch chan int) {
	for {
		select {
		case i := <-ch:
			fmt.Println("this  value of unbuffer channel", i)
		}
	}
}
```
执行结果：
```shell
# go run main.go 
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [chan send]:
main.main()
        /Users/gaofubao/go/src/cookbook-go/golang/channel/ch1/main.go:10 +0x37
exit status 2
```
这里会报错 fatal error: all goroutines are asleep - deadlock! 就是因为 ch<-1 发送了，但是同时没有接收者，所以就发生了阻塞，但如果我们把 ch <- 1 放到 go loop(ch) 下面，程序就会正常运行。

缓冲 channel：
```go
func main() {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3
	ch <- 4
	go loop(ch)
	time.Sleep(1 * time.Millisecond)
}

func loop(ch chan int) {
	for {
		select {
		case i := <-ch:
			fmt.Println("this value of unbuffer channel", i)
		}
	}
}
```
执行结果：
```shell
# go run main.go
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [chan send]:
main.main()
        /Users/gaofubao/go/src/cookbook-go/golang/channel/ch2/main.go:13 +0x6d
exit status 2
```
这里也会报 fatal error: all goroutines are asleep - deadlock!，这是因为 channel 的大小为 3 ，而我们要往里面塞 4 个数据，所以就会阻塞住，解决的办法有两个:
1. 把 channel 长度调大一点
2. 把 channel 的信息发送者 ch <- 1 这些代码移动到 go loop(ch) 下面，让 channel 实时消费就不会导致阻塞了

## channel 特点
---
channel 有 3 种模式：写操作模式（单向通道）、读操作模式（单向通道）、读写操作模式（双向通道）

|      | 写操作模式       | 读操作模式       | 读写操作模式   |
| ---- | ---------------- | ---------------- | -------------- |
| 创建 | make(chan<- int) | make(<-chan int) | make(chan int) |

channel有3种状态：未初始化、正常、关闭

|      | 未初始           | 关闭                               | 正常             |
| ---- | ---------------- | ---------------------------------- | ---------------- |
| 关闭 | panic            | panic                              | 正常关闭         |
| 发送 | 永远阻塞导致死锁 | panic                              | 阻塞或者成功发送 |
| 接收 | 永远阻塞导致死锁 | 缓冲区为空则为零值, 否则可以继续读 | 阻塞或者成功接收 |

注意事项：
- 一个 channel 不能多次关闭，会导致 painc。
- 如果多个 goroutine 都监听同一个 channel，那么 channel 上的数据都可能随机被某一个 goroutine 取走进行消费。
- 如果多个 goroutine 监听同一个 channel，如果这个 channel 被关闭，则所有 goroutine 都能收到退出信号。

## channel 线程安全
---
为什么设计成线程安全？
不同协程通过 channel 进行通信，本身的使用场景就是多线程，为了保证数据的一致性，必须实现线程安全。

如何实现线程安全的？
channel 的底层实现中，hchan 结构体中采用 Mutex 锁来保证数据读写安全。在对循环数组 buf 中的数据进行入队和出队操作时，必须先获取互斥锁，才能操作 channel 数据。

## channel 控制 goroutine 并发顺序
---

多个 goroutine 并发执行时，每一个 goroutine 抢到处理器的时间点不一致，gorouine 的执行本身不能保证顺序，即代码中先写的 gorouine 并不能保证先执行。
思路：使用 channel 进行通信通知，用 channel 去传递信息，从而控制并发执行顺序

```go
func main() {
	ch1 := make(chan struct{}, 1)
	ch2 := make(chan struct{}, 1)
	ch3 := make(chan struct{}, 1)
	
	ch1 <- struct{}{}
	wg.Add(3)
	start := time.Now().Unix()
	go print("gorouine1", ch1, ch2)
	go print("gorouine2", ch2, ch3)
	go print("gorouine3", ch3, ch1)
	wg.Wait()	
	end := time.Now().Unix()
	fmt.Printf("duration:%d\n", end-start)
}

func print(gorouine string, inputchan chan struct{}, outchan chan struct{}) {
	// 模拟内部操作耗时
	time.Sleep(1 * time.Second)
	select {
	case <-inputchan:
		fmt.Printf("%s\n", gorouine)
		outchan <- struct{}{}
	}
	wg.Done()
}
```

## channel 共享内存优劣
---

“不要通过共享内存来通信，我们应该使用通信来共享内存” 这句话想必大家已经非常熟悉了，在官方的博客，初学时的教程，甚至是在 Go 的源码中都能看到。

无论是通过共享内存来通信还是通过通信来共享内存，最终我们应用程序都是读取的内存当中的数据，只是前者是直接读取内存的数据，而后者是通过发送消息的方式来进行同步。
而通过发送消息来同步的这种方式常见的就是 Go 采用的 CSP(Communication Sequential Process) 模型以及 Erlang 采用的 Actor 模型，这两种方式都是通过通信来共享内存。

![共享内存](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041937232.png)

大部分的语言采用的都是第一种方式直接去操作内存，然后通过互斥锁，CAS 等操作来保证并发安全。Go 引入了 Channel 和 Goroutine 实现 CSP 模型将生产者和消费者进行了解耦，Channel 其实和消息队列很相似。而 Actor 模型和 CSP 模型都是通过发送消息来共享内存，但是它们之间最大的区别就是 Actor 模型当中并没有一个独立的 Channel 组件，而是 Actor 与 Actor 之间直接进行消息的发送与接收，每个 Actor 都有一个本地的“信箱”消息都会先发送到这个“信箱当中”。

- 优点：使用 channel 可以帮助我们解耦生产者和消费者，可以降低并发当中的耦合
- 缺点：容易出现死锁的情况

## channel 死锁
---

死锁：
- 单个协程永久阻塞
- 两个或两个以上的协程的执行过程中，由于竞争资源或由于彼此通信而造成的一种阻塞的现象

channel 死锁场景：
- 非缓存 channel 只写不读
- 非缓存 channel 读在写后面
- 缓存 channel 写入超过缓冲区数量
- 空读
- 多个协程互相等待

1. 非缓存 channel 只写不读
```go
func deadlock() {
    ch := make(chan int) 
    ch <- 3 //  这里会发生一直阻塞的情况，执行不到下面一句
}
```

2. 非缓存 channel 读在写后面
```go
func deadlock() {
    ch := make(chan int)
    ch <- 3  //  这里会发生一直阻塞的情况，执行不到下面一句
    num := <-ch
    fmt.Println("num=", num)
}

func deadlock() {
    ch := make(chan int)
    ch <- 100 //  这里会发生一直阻塞的情况，执行不到下面一句
    go func() {
        num := <-ch
        fmt.Println("num=", num)
    }()
    time.Sleep(time.Second)
}
```

3. 缓存 channel 写入超过缓冲区数量
```go
func deadlock() {
    ch := make(chan int, 3)
    ch <- 3
    ch <- 4
    ch <- 5
    ch <- 6  //  这里会发生一直阻塞的情况
}
```

4. 空读
```go
func deadlock() {
    ch := make(chan int)
    // ch := make(chan int, 1)
    fmt.Println(<-ch)  //  这里会发生一直阻塞的情况
}
```

5. 多个协程互相等待
```go
func deadlock5() {
    ch1 := make(chan int)
    ch2 := make(chan int)
    
    // 互相等对方造成死锁
    go func() {
        for {
            select {
            case num := <-ch1:
                fmt.Println("num=", num)
                ch2 <- 100
            }
        }
    }()

    for {
        select {
        case num := <-ch2:
            fmt.Println("num=", num)
            ch1 <- 300
        }
    }
}
```
