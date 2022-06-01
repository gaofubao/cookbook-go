# 调度模型

## 线程模型
Go 实现的是两级线程模型（M:N)，准确的说是 GMP 模型，是对两级线程模型的改进实现，使它能够更加灵活地进行线程之间的调度。

### 背景
- 单进程时代：每个程序就是一个进程，直到一个程序运行完，才能进行下一个进程	
    - 无法并发，只能串行 
    - 进程阻塞所带来的 CPU 时间浪费
- 多进程/线程时代：一个线程阻塞，CPU 可以立刻切换到其他线程中去执行
    - 进程/线程占用内存高
    - 进程/线程上下文切换成本高
- 协程时代：协程（用户态线程）绑定线程（内核态线程），CPU 调度线程执行
    - 实现起来较复杂，协程和线程的绑定依赖调度器算法

线程 -> CPU 由操作系统调度，协程 -> 线程由 Go 调度器来调度，协程与线程的映射关系有三种线程模型。

### 三种线程模型
线程实现模型主要分为：内核级线程模型、用户级线程模型、两级线程模型，他们的区别在于用户线程与内核线程之间的对应关系。

#### 内核级线程模型（1:1）
1 个用户线程对应 1 个内核线程，这种最容易实现，协程的调度都由 CPU 完成了。
![内核级线程模型](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206011145742.jpeg)

内核级线程模型的优缺点：
- 优点
    - 实现起来最简单
    - 能够利用多核
    - 如果进程中的一个线程被阻塞，不会阻塞其他线程，是能够切换同一进程内的其他线程继续执行
- 缺点
    - 上下文切换成本高，创建、删除和切换都由 CPU 完成

#### 用户级线程模型（N:1）
1 个进程中的所有线程对应 1 个内核线程。
![用户级线程模型](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206011145883.jpeg)

用户级线程模型的优缺点：
- 优点
    - 上下文切换成本低，在用户态即可完成协程切换
- 缺点
    - 无法利用多核
    - 一旦协程阻塞，造成线程阻塞，本线程的其它协程无法执行

#### 两级线程模型（M:N)
M 个线程对应 N 个内核线程。
![两级线程模型](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206011146726.jpeg)

两级线程模型的优缺点：
- 优点
    - 能够利用多核
    - 上下文切换成本低
    - 如果进程中的一个线程被阻塞，不会阻塞其他线程，是能够切换同一进程内的其他线程继续执行
- 缺点
    - 实现起来最复杂

## Go 调度模型
什么才是一个好的调度器？能在适当的时机将合适的协程分配到合适的位置，保证公平和效率。Go 采用了 GMP 模型（对两级线程模型的改进实现），使它能够更加灵活地进行线程之间的调度。

### GM 模型
Go 早期是 GM 模型，没有 P 组件。
![GM 模型](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206011148284.png)

GM调度存在的问题：
1. 全局队列的锁竞争，当 M 从全局队列中添加或者获取 G 的时候，都需要获取队列锁，导致激烈的锁竞争
2. M 转移 G 增加额外开销，当 M1 在执行 G1 的时候，M1 创建了 G2，为了继续执行 G1，需要把 G2 保存到全局队列中，无法保证 G2 是被 M1 处理。因为 M1 原本就保存了 G2 的信息，所以 G2 最好是在 M1 上执行，这样的话也不需要转移 G 到全局队列和线程上下文切换
3. 线程使用效率不能最大化，没有 work-stealing 和 hand-off 机制

### GMP 模型
GMP 是 Go 运行时调度层面的实现，包含 4 个重要结构，分别是 G、M、P、Sched。
![GMP 模型](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206011151586.png)

- G（Goroutine）：代表 Go 协程 Goroutine，存储了 Goroutine 的执行栈信息、Goroutine 状态以及 Goroutine 的任务函数等。G 的数量无限制，理论上只受内存的影响，创建一个 G 的初始栈大小为 2-4K，配置一般的机器也能简简单单开启数十万个 Goroutine ，而且 Go 语言在 G 退出的时候还会把 G 清理之后放到 P 本地或者全局的闲置列表 gFree 中以便复用。
- M（Machine）：Go 对操作系统线程（OS thread）的封装，可以看作操作系统内核线程，想要在 CPU 上执行代码必须有线程，通过系统调用 clone 创建。M在绑定有效的 P 后，进入一个调度循环，而调度循环的机制大致是从 P 的本地运行队列以及全局队列中获取 G，切换到 G 的执行栈上并执行 G 的函数，调用 goexit 做清理工作并回到 M，如此反复。M 并不保留 G 状态，这是 G 可以跨 M 调度的基础。M 的数量有限制，默认数量限制是 10000，可以通过 debug.SetMaxThreads() 方法进行设置，如果有 M 空闲，那么就会回收或者睡眠。
- P（Processor）：虚拟处理器，M 执行 G 所需要的资源和上下文，只有将 P 和 M 绑定，才能让 P 的 runq 中的 G 真正运行起来。P 的数量决定了系统内最大可并行的 G 的数量，P的数量受本机的 CPU 核数影响，可通过环境变量 $GOMAXPROCS 或在 runtime.GOMAXPROCS() 来设置，默认为 CPU 核心数。
- Sched：调度器结构，它维护有存储 M 和 G 的全局队列，以及调度器的一些状态信息。

|          | G                      | M                                                            | P                                                            |
| -------- | ---------------------- | :----------------------------------------------------------- | :----------------------------------------------------------- |
| 数量限制 | 无限制，受机器内存影响 | 有限制，默认最多 10000                                       | 有限制，最多 GOMAXPROCS 个                                   |
| 创建时机 | go func                | 当没有足够的 M 来关联 P 并运行其中的可运行的 G 时会请求创建新的 M | 在确定了 P 的最大数量 n 后，运行时系统会根据这个数量创建个 P |

核心数据结构：
```go
// /src/runtime/runtime2.go
type g struct {
    goid    int64   // 唯一的goroutine的ID
    sched   gobuf   // goroutine切换时，用于保存g的上下文
    stack   stack   // 栈
    gopc            // pc of go statement that created this goroutine
    startpc uintptr // pc of goroutine function
    // ...
}

type p struct {
    lock mutex
    id      int32
    status  uint32 // one of pidle/prunning/...

    // Queue of runnable goroutines. Accessed without lock.
    runqhead uint32 // 本地队列队头
    runqtail uint32 // 本地队列队尾
    // 本地队列，大小256的数组，数组往往会被都读入到缓存中，对缓存友好，效率较高
    runq     [256]guintptr 
    // 下一个优先执行的goroutine（一定是最后生产出来的)，
    // 为了实现局部性原理，runnext中的G永远会被最先调度执行
    runnext guintptr 
    // ... 
}

type m struct {
    g0      *g     
    // 每个M都有一个自己的G0，不指向任何可执行的函数，
    // 在调度或系统调用时，M会切换到G0，使用G0的栈空间来调度
    curg    *g    
    // 当前正在执行的G
    // ... 
}

type schedt struct {
    runq        gQueue // 全局队列，链表（长度无限制）
    runqsize    int32  // 全局队列长度
    // ...
}
```

GMP 模型的实现算是 Go 调度器的一大进步，但调度器仍然有一个令人头疼的问题，那就是不支持抢占式调度，这导致一旦某个 G 中出现死循环的代码逻辑，那么 G 将永久占用分配给它的 P 和 M，而位于同一个 P 中的其他 G 将得不到调度，出现“饿死”的情况。
当只有一个 P（GOMAXPROCS=1）时，整个 Go 程序中的其他 G 都将“饿死”。于是在 Go 1.2 版本中实现了基于协作的“抢占式”调度，在 Go 1.14 版本中实现了基于信号的“抢占式”调度。

计算机科学领域的任何问题都可以通过增加一个间接的中间层来解决，为了解决这一的问题 Go 从 1.1 版本引入 P，在运行时中加入 P 对象，让 P 去管理这个 G 对象，M 想要运行 G，必须绑定 P，才能运行 P 所管理的 G。

## Go 调度原理
Goroutine 调度的本质就是将 Goroutine (G）按照一定算法放到 CPU 上去执行。
CPU 感知不到 Goroutine，只知道内核线程，所以需要 Go 调度器将协程调度到内核线程上面去，然后操作系统调度器将内核线程放到 CPU 上去执行。

M 是对内核级线程的封装，所以 Go 调度器的工作就是将 G 分配到 M。

Go 调度器的实现不是一蹴而就的，它的调度模型与算法也是几经演化，从最初的 GM 模型、到 GMP 模型，从不支持抢占，到支持协作式抢占，再到支持基于信号的异步抢占，经历了不断地优化与打磨。

### 设计思想
- 线程复用（work stealing 机制和 hand off 机制）
- 利用并行（利用多核 CPU）
- 抢占调度（解决公平性问题）

### 调度对象
Go 调度器
> Go 调度器是属于 Go Runtime 中的一部分，Go Runtime 负责实现 Go 的并发调度、垃圾回收、内存堆栈管理等关键功能。

### 被调度对象
- G 的来源
    - P 的 runnext（只有 1 个 G，局部性原理，永远会被最先调度执行）
    - P 的本地队列（数组，最多 256 个 G）
    - 全局 G 队列（链表，无限制）
    - 网络轮询器 network poller（存放网络调用被阻塞的 G）
- P 的来源
    - 全局 P 队列（数组，GOMAXPROCS 个 P）
- M 的来源
    - 休眠线程队列（未绑定 P，长时间休眠会等待 GC 回收销毁）
    - 运行线程（绑定 P，指向 P 中的 G）
    - 自旋线程（绑定 P，指向 M 的 G0）

其中运行线程数 + 自旋线程数 <= P 的数量（GOMAXPROCS），M 个数 >= P 个数

### 调度流程
协程的调度采用了生产者-消费者模型，实现了用户任务与调度器的解耦。
![调度流程](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206011220239.png)

生产端我们开启的每个协程都是一个计算任务，这些任务会被提交给 go 的 runtime。如果计算任务非常多，有成千上万个，那么这些任务是不可能同时被立刻执行的，所以这个计算任务一定会被先暂存起来，一般的做法是放到内存的队列中等待被执行。

G 的生命周期：G 从创建、保存、被获取、调度和执行、阻塞、销毁，过程如下：
![调度流程](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206012023826.jpeg)
1. 创建 G，关键字 go func() 创建 G。
2. 保存 G，创建的 G 优先保存到本地队列 P，如果 P 满了，则会平衡部分 P 到全局队列中。
3. 唤醒或者新建 M 执行任务，进入调度循环（4,5,6)。
4. M 获取 G，M 首先从 P 的本地队列获取 G，如果 P 为空，则从全局队列获取 G，如果全局队列也为空，则从另一个本地队列偷取一半数量的 G（负载均衡），这种从其它P偷的方式称之为 work stealing。
5. M 调度和执行 G，M 调用 G.func() 函数执行 G
    - 如果 M 在执行 G 的过程发生系统调用阻塞（同步），会阻塞 G 和 M（操作系统限制），此时 P 会和当前 M 解绑，并寻找新的 M，如果没有空闲的 M 就会新建一个 M ，接管正在阻塞 G 所属的 P，接着继续执行 P 中其余的 G，这种阻塞后释放 P 的方式称之为 hand off。当系统调用结束后，这个 G 会尝试获取一个空闲的 P 执行，优先获取之前绑定的 P，并放入到这个 P 的本地队列，如果获取不到 P，那么这个线程 M 变成休眠状态，加入到空闲线程中，然后这个 G 会被放入到全局队列中。
    - 如果 M 在执行 G 的过程发生网络 IO 等操作阻塞时（异步），阻塞 G，不会阻塞 M。M 会寻找 P 中其它可执行的 G 继续执行，G 会被网络轮询器 network poller 接手，当阻塞的 G 恢复后，G1 从 network poller 被移回到 P 的 LRQ 中，重新进入可执行状态。异步情况下，通过调度，Go scheduler 成功地将 I/O 的任务转变成了 CPU 任务，或者说将内核级别的线程切换转变成了用户级别的 goroutine 切换，大大提高了效率。
6. M 执行完 G 后清理现场，重新进入调度循环（将 M 上运⾏的 goroutine 切换为 G0，G0 负责调度时协程的切换）。

其中 2 中保存 G 的详细流程如下：
![G 保存](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206012021803.png)
- 执行 go func 的时候，主线程 M0 会调用 newproc() 生成一个 G 结构体，这里会先选定当前 M0 上的 P 结构。
- 每个协程 G 都会被尝试先放到 P 中的 runnext，若 runnext 为空则放到 runnext 中，生产结束。
- 若 runnext 满，则将原来 runnext 中的 G 踢到本地队列中，将当前 G 放到 runnext 中，生产结束。
- 若本地队列也满了，则将本地队列中的 G 拿出一半，放到全局队列中，生产结束。

### 调度时机
什么时候进行调度（执行/切换）？在以下情形下，会切换正在执行的 goroutine：
- 抢占式调度
    - sysmon 检测到协程运行过久（比如 sleep，死循环）
    - 切换到 g0，进入调度循环
- 主动调度
    - 新起一个协程和协程执行完毕
        - 触发调度循环
    - 主动调用 runtime.Gosched()
        - 切换到 g0，进入调度循环
    - 垃圾回收之后
        - stw 之后，会重新选择 g 开始执行
- 被动调度
    - 系统调用（比如文件 IO）阻塞（同步）
        - 阻塞 G 和 M，P 与 M 分离，将 P 交给其它 M 绑定，其它 M 执行 P 的剩余 G
    - 网络 IO 调用阻塞（异步）
        - 阻塞 G，G 移动到 NetPoller，M 执行 P 的剩余 G
    - atomic/mutex/channel 等阻塞（异步）
        - 阻塞 G，G 移动到 channel 的等待队列中，M 执行 P 的剩余 G

### 调度策略
使用什么策略来挑选下一个 goroutine 执行？
由于 P 中的 G 分布在 runnext、本地队列、全局队列、网络轮询器中，则需要挨个判断是否有可执行的 G，大体逻辑如下：
- 每执行 61 次调度循环，从全局队列获取 G，若有则直接返回
- 从 P 上的 runnext 看一下是否有 G，若有则直接返回
- 从 P 上的 本地队列 看一下是否有 G，若有则直接返回
- 上面都没查找到时，则去全局队列、网络轮询器查找或者从其他 P 中窃取，一直阻塞直到获取到一个可用的 G 为止

源码实现如下：
```go
func schedule() {
    _g_ := getg()
    var gp *g
    var inheritTime bool
    // ...
    if gp == nil {
        // 每执行61次调度循环会看一下全局队列。
        // 为了保证公平，避免全局队列一直无法得到执行的情况，当全局运行队列中有待执行的G时，
        // 通过schedtick保证有一定几率会从全局的运行队列中查找对应的Goroutine；
        if _g_.m.p.ptr().schedtick%61 == 0 && sched.runqsize > 0 {
            lock(&sched.lock)
            gp = globrunqget(_g_.m.p.ptr(), 1)
            unlock(&sched.lock)
        }
    }
    if gp == nil {
        // 先尝试从P的runnext和本地队列查找G
        gp, inheritTime = runqget(_g_.m.p.ptr())
    }
    if gp == nil {
        // 仍找不到，去全局队列中查找。
        // 还找不到，要去网络轮询器中查找是否有G等待运行；
        // 仍找不到，则尝试从其他P中窃取G来执行。
        gp, inheritTime = findrunnable() // blocks until work is available
        // 这个函数是阻塞的，执行到这里一定会获取到一个可执行的G
    }
    // ...
    // 调用execute，继续调度循环
    execute(gp, inheritTime)
}
```

从全局队列查找时，如果要所有 P 平分全局队列中的 G，每个 P 要分得多少个，这里假设会分得 n 个。然后把这 n 个 G，转移到当前 G 所在 P 的本地队列中去。但是最多不能超过 P 本地队列长度的一半（即 128）。这样做的目的是，如果下次调度循环到来的时候，就不必去加锁到全局队列中在获取一次 G 了，性能得到了很好的保障。
```go
func globrunqget(_p_ *p, max int32) *g {
   ...
   // gomaxprocs = p的数量
   // sched.runqsize是全局队列长度
   // 这里n = 全局队列的G平分到每个P本地队列上的数量 + 1
   n := sched.runqsize/gomaxprocs + 1
   if n > sched.runqsize {
      n = sched.runqsize
   }
   if max > 0 && n > max {
      n = max
   }
   // 平分后的数量n不能超过本地队列长度的一半，也就是128
   if n > int32(len(_p_.runq))/2 {
      n = int32(len(_p_.runq)) / 2
   }

   // 执行将G从全局队列中取n个分到当前P本地队列的操作
   sched.runqsize -= n

   gp := sched.runq.pop()
   n--
   for ; n > 0; n-- {
      gp1 := sched.runq.pop()
      runqput(_p_, gp1, false)
   }
   return gp
}
```
从其它 P 查找时，会偷一半的 G 过来放到当前 P 的本地队列。

## work stealing 机制

## hand off 机制

## 抢占式调度

## 查看运行时调度信息
有两种方式可以查看一个程序调度信息，分别是 go tool trace 和 GODEBUG。

### go tool trace

### GODEBUG 
GODEBUG 变量可以控制运行时内的调试变量。查看调度器信息，将会使用如下两个参数：
- schedtrace：设置 schedtrace=X 参数可以使运行时在每 X 毫秒发出一行调度器的摘要信息到标准 err 输出中。
- scheddetail：设置 schedtrace=X 和 scheddetail=1 可以使运行时在每 X 毫秒发出一次详细的多行信息，信息内容主要包括调度程序、处理器、OS 线程 和 Goroutine 的状态。
