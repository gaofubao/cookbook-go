# 调度模型

## 线程模型
---
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
---
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
---
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
---
### 概述
当线程 M ⽆可运⾏的 G 时，尝试从其他 M 绑定的 P 中偷取 G，减少空转，提高了线程利用率（避免闲着不干活）。
![work stealing](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206031702018.png)
当从本线程绑定 P 本地队列、全局 G 队列、netpoller 都找不到可执行的 G，会从别的 P 里窃取 G 并放到当前 P 上面。
- 从 netpoller 中拿到的 G 是 _Gwaiting 状态（存放的是因为网络 IO 被阻塞的 G），从其它地方拿到的 G 是 _Grunnable 状态。
- 从全局队列取的 G 数量：N = min(len(GRQ)/GOMAXPROCS + 1, len(GRQ/2))（根据 GOMAXPROCS 负载均衡），从其它 P 本地队列窃取的 G 数量：N = len(LRQ)/2（平分）。

### 窃取流程
源码见 runtime/proc.go 中的 stealWork 函数，窃取流程如下，如果经过多次努力一直找不到需要运行的 goroutine 则调用 stopm 进入睡眠状态，等待被其它工作线程唤醒。
1. 选择要窃取的 P
2. 从 P 中偷走一半 G

#### 选择要窃取的 P
窃取的实质就是遍历 allp 中的所有 p，查看其运行队列是否有 goroutine，如果有，则取其一半到当前工作线程的运行队列。
为了保证公平性，遍历 allp 时并不是固定的从 allp[0] 即第一个 p 开始，而是从随机位置上的 p 开始，而且遍历的顺序也随机化了，并不是现在访问了第 i 个 p 下一次就访问第 i+1 个 p，而是使用了一种伪随机的方式遍历 allp 中的每个 p，防止每次遍历时使用同样的顺序访问 allp 中的元素。
```go
offset := uint32(random()) % nprocs
coprime := 随机选取一个小于nprocs且与nprocs互质的数
const stealTries = 4 // 最多重试4次
for i := 0; i < stealTries; i++ {
    for i := 0; i < nprocs; i++ {
      p := allp[offset]
        从p的运行队列偷取goroutine
        if 偷取成功 {
        break
     }
        offset += coprime
        offset = offset % nprocs
     }
}
```
可以看到只要随机数不一样，偷取 p 的顺序也不一样，但可以保证经过 nprocs 次循环，每个 p 都会被访问到。

##### 从 P 中偷走一半 G
源码见 runtime/proc.go 中的 runqsteal 函数：
挑选出盗取的对象 p 之后，则调用 runqsteal 盗取 p 的运行队列中的 goroutine，runqsteal 函数再调用 runqgrap 从 p 的本地队列尾部批量偷走一半的 g。
为啥是偷一半的 g，可以理解为负载均衡。
```go
func runqgrab(_p_ *p, batch *[256]guintptr, batchHead uint32, stealRunNextG bool) uint32 {
    for {
        h := atomic.LoadAcq(&_p_.runqhead)  // load-acquire, synchronize with other consumers
        t := atomic.LoadAcq(&_p_.runqtail)  // load-acquire, synchronize with the producer
        n := t - h                          //计算队列中有多少个goroutine
        n = n - n/2                         //取队列中goroutine个数的一半
        if n == 0 {
            ......
            return ......
        }
        return n
    }
}
```

## hand off 机制
---
### 概述
hand off 机制也称为 P 分离机制，当本线程 M 因为 G 进行的系统调用阻塞时，线程释放绑定的 P，把 P 转移给其他空闲的 M 执行，也提高了线程利用率（避免站着茅坑不拉 shi）。

### 分离流程
当前线程 M 阻塞时，释放 P，给其它空闲的 M 处理。
![hand off](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206031711834.png)

## 抢占式调度
---
在 1.2 版本之前，Go 的调度器仍然不支持抢占式调度，程序只能依靠 Goroutine 主动让出 CPU 资源才能触发调度，这会引发一些问题，比如：
- 某些 Goroutine 可以长时间占用线程，造成其它 Goroutine 的饥饿
- 垃圾回收器是需要 stop the world 的，如果垃圾回收器想要运行了，那么它必须先通知其它的 Goroutine 停下来，这会造成较长时间的等待时间

为解决这个问题：
- Go 1.2 中实现了基于协作的“抢占式”调度
- Go 1.14 中实现了基于信号的“抢占式”调度

### 基于协作的抢占式调度
- 协作式：大家都按事先定义好的规则来，比如：一个 Goroutine 执行完后，退出，让出 p，然后下一个 Goroutine 被调度到 p 上运行。这样做的缺点就在于是否让出 p 的决定权在 Groutine 自身。一旦某个 g 不主动让出 p 或执行时间较长，那么后面的 Goroutine 只能等着，没有方法让前者让出 p，导致延迟甚至饿死。
- 非协作式：就是由 runtime 来决定一个 Goroutine 运行多长时间，如果你不主动让出，对不起，我有手段可以抢占你，把你踢出去，让后面的 Goroutine 进来运行。

基于协作的抢占式调度流程：
- 编译器会在调用函数前插入 runtime.morestack，让运行时有机会在这段代码中检查是否需要执行抢占调度
- Go 语言运行时会在垃圾回收暂停程序、系统监控发现 Goroutine 运行超过 10ms，那么会在这个协程设置一个抢占标记
- 当发生函数调用时，可能会执行编译器插入的 runtime.morestack，它调用的 runtime.newstack 会检查抢占标记，如果有抢占标记就会触发抢占让出 CPU，切到调度主协程里

这种解决方案只能说局部解决了“饿死”问题，只在有函数调用的地方才能插入“抢占”代码（埋点），对于没有函数调用而是纯算法循环计算的 G，Go 调度器依然无法抢占。

比如，死循环等并没有给编译器插入抢占代码的机会，以下程序在 go 1.14 之前的 go 版本中，运行后会一直卡住，而不会打印 I got scheduled!
```go
func main() {
    runtime.GOMAXPROCS(1)
    go func() {
        for {
        }
    }()

    time.Sleep(time.Second)
    fmt.Println("I got scheduled!")
}
```
为了解决这些问题，Go 在 1.14 版本中增加了对非协作的抢占式调度的支持，这种抢占式调度是基于系统信号的，也就是通过向线程发送信号的方式来抢占正在运行的 Goroutine。

### 基于信号的抢占式调度
真正的抢占式调度是基于信号完成的，所以也称为“异步抢占”。不管协程有没有意愿主动让出 CPU 运行权，只要某个协程执行时间过长，就会发送信号强行夺取 CPU 运行权。
- M 注册一个 SIGURG 信号的处理函数：sighandler
- sysmon 启动后会间隔性的进行监控，最长间隔 10ms，最短间隔 20us。如果发现某协程独占 P 超过 10ms，会给 M 发送抢占信号
- M 收到信号后，内核执行 sighandler 函数把当前协程的状态从 _Grunning 正在执行改成 _Grunnable 可执行，把抢占的协程放到全局队列里，M 继续寻找其他 Goroutine 来运行
- 被抢占的 G 再次调度过来执行时，会继续原来的执行流

抢占分为 _Prunning 和 _Psyscall，_Psyscall 抢占通常是由于阻塞性系统调用引起的，比如磁盘 io、cgo。_Prunning 抢占通常是由于一些类似死循环的计算逻辑引起的。

## 查看运行时调度信息
---
有两种方式可以查看一个程序调度信息，分别是 go tool trace 和 GODEBUG。
```go
func main() {
	// 创建trace文件
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// 启动trace goroutine
	err = trace.Start(f)
	if err != nil {
		panic(err)
	}
	defer trace.Stop()

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		fmt.Println("Hello World")
	}
}
```

### go tool trace
```go
# go run main.go

# go tool trace trace.out
2022/06/03 18:10:04 Parsing trace...
2022/06/03 18:10:04 Splitting trace...
2022/06/03 18:10:04 Opening browser. Trace viewer is listening on http://127.0.0.1:55596
```

#### 查看可视化界面
![trace](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206031816835.png)

点击 view trace 能够看见可视化的调度流程：
![view trace](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206031817004.png)
一共有2个 G 在程序中，一个是特殊的 G0，是每个 M 必须有的一个初始化的 G，另外一个是 G1 main goroutine (执行 main 函数的协程)，在一段时间内处于可运行和运行的状态。

#### 查看 M 详细信息
点击 Threads 那一行可视化的数据条查看：
![Threads](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206031823510.png)
一共有 2 个 M 在程序中，一个是特殊的 M0，用于初始化使用，另外一个是用于执行 G1 的 M1。

#### 查看 P 上正在运行的 Goroutine 详细信息
点击 Proc 那一行可视化的数据条查看：
![Proc](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206031830554.png)
一共有 3 个 P 在程序中，分别是 P0、P1、P2。

点击具体的 Goroutine 行为后可以看到其相关联的详细信息:
- Start：开始时间
- Wall Duration：持续时间
- Self Time：执行时间
- Start Stack Trace：开始时的堆栈信息
- End Stack Trace：结束时的堆栈信息
- Incoming flow：输入流
- Outgoing flow：输出流
- Preceding events：之前的事件
- Following events：之后的事件
- All connected：所有连接的事件

### GODEBUG 
GODEBUG 变量可以控制运行时内的调试变量。查看调度器信息，将会使用如下两个参数：
- schedtrace：设置 schedtrace=X 参数可以使运行时在每 X 毫秒发出一行调度器的摘要信息到标准 err 输出中。
- scheddetail：设置 schedtrace=X 和 scheddetail=1 可以使运行时在每 X 毫秒发出一次详细的多行信息，信息内容主要包括调度程序、处理器、OS 线程 和 Goroutine 的状态。

#### 查看基本信息
```go
# go build main.go 
# GODEBUG=schedtrace=1000 ./main 
SCHED 0ms: gomaxprocs=8 idleprocs=5 threads=5 spinningthreads=1 idlethreads=0 runqueue=0 [1 0 0 0 0 0 0 0]
Hello World
SCHED 1005ms: gomaxprocs=8 idleprocs=8 threads=5 spinningthreads=0 idlethreads=3 runqueue=0 [0 0 0 0 0 0 0 0]
Hello World
SCHED 2013ms: gomaxprocs=8 idleprocs=8 threads=5 spinningthreads=0 idlethreads=3 runqueue=0 [0 0 0 0 0 0 0 0]
Hello World
SCHED 3018ms: gomaxprocs=8 idleprocs=8 threads=5 spinningthreads=0 idlethreads=3 runqueue=0 [0 0 0 0 0 0 0 0]
```
- sched：每一行都代表调度器的调试信息，后面提示的毫秒数表示启动到现在的运行时间，输出的时间间隔受 schedtrace 的值影响。
- gomaxprocs：当前的 CPU 核心数（GOMAXPROCS 的当前值）。
- idleprocs：空闲的处理器数量，后面的数字表示当前的空闲数量。
- threads：OS 线程数量，后面的数字表示当前正在运行的线程数量。
- spinningthreads：自旋状态的 OS 线程数量。
- idlethreads：空闲的线程数量。
- runqueue：全局队列中中的 Goroutine 数量，而后面的 [0 0 0 0 0 0 0 0] 则分别代表这 8 个 P 的本地队列正在运行的 Goroutine 数量。

#### 查看详细信息
```go
# go build main.go 
# GODEBUG=scheddetail=1,schedtrace=1000 ./main 
SCHED 0ms: gomaxprocs=8 idleprocs=6 threads=4 spinningthreads=1 idlethreads=0 runqueue=0 gcwaiting=0 nmidlelocked=0 stopwait=0 sysmonwait=0
  P0: status=0 schedtick=0 syscalltick=0 m=-1 runqsize=1 gfreecnt=0 timerslen=0
  P1: status=1 schedtick=0 syscalltick=0 m=2 runqsize=0 gfreecnt=0 timerslen=0
  P2: status=0 schedtick=0 syscalltick=0 m=-1 runqsize=0 gfreecnt=0 timerslen=0
  P3: status=0 schedtick=0 syscalltick=0 m=-1 runqsize=0 gfreecnt=0 timerslen=0
  P4: status=0 schedtick=0 syscalltick=0 m=-1 runqsize=0 gfreecnt=0 timerslen=0
  P5: status=0 schedtick=0 syscalltick=0 m=-1 runqsize=0 gfreecnt=0 timerslen=0
  P6: status=0 schedtick=0 syscalltick=0 m=-1 runqsize=0 gfreecnt=0 timerslen=0
  P7: status=0 schedtick=0 syscalltick=0 m=-1 runqsize=0 gfreecnt=0 timerslen=0
  M3: p=0 curg=-1 mallocing=0 throwing=0 preemptoff= locks=1 dying=0 spinning=false blocked=false lockedg=-1
  M2: p=1 curg=-1 mallocing=0 throwing=0 preemptoff= locks=2 dying=0 spinning=false blocked=false lockedg=-1
  M1: p=-1 curg=-1 mallocing=0 throwing=0 preemptoff= locks=2 dying=0 spinning=false blocked=false lockedg=-1
  M0: p=-1 curg=-1 mallocing=0 throwing=0 preemptoff= locks=1 dying=0 spinning=false blocked=false lockedg=1
  G1: status=1(chan receive) m=-1 lockedm=0
  G2: status=1() m=-1 lockedm=-1
  G3: status=1() m=-1 lockedm=-1
  G4: status=4(GC scavenge wait) m=-1 lockedm=-1
```

G：
- status：G 的运行状态
- m：隶属哪一个 M
- lockedm：是否有锁定 M

G 的 9 种运行状态：

| 状态              | 值   | 含义                                                         |
| ----------------- | ---- | ------------------------------------------------------------ |
| _Gidle            | 0    | 刚刚被分配，还没有进行初始化。                               |
| _Grunnable        | 1    | 已经在运行队列中，还没有执行用户代码。                       |
| _Grunning         | 2    | 不在运行队列里中，已经可以执行用户代码，此时已经分配了 M 和 P。 |
| _Gsyscall         | 3    | 正在执行系统调用，此时分配了 M。                             |
| _Gwaiting         | 4    | 在运行时被阻止，没有执行用户代码，也不在运行队列中，此时它正在某处阻塞等待中。 |
| _Gmoribund_unused | 5    | 尚未使用，但是在 gdb 中进行了硬编码。                        |
| _Gdead            | 6    | 尚未使用，这个状态可能是刚退出或是刚被初始化，此时它并没有执行用户代码，有可能有也有可能没有分配堆栈。 |
| _Genqueue_unused  | 7    | 尚未使用。                                                   |
| _Gcopystack       | 8    | 正在复制堆栈，并没有执行用户代码，也不在运行队列中。         |

M：
- p：隶属哪一个 P
- curg：当前正在使用哪个 G
- runqsize：运行队列中的 G 数量
- gfreecnt：可用的 G（状态为 Gdead）
- mallocing：是否正在分配内存
- throwing：是否抛出异常
- preemptoff：不等于空字符串的话，保持 curg 在这个 m 上运行

P：
- status：P 的运行状态
- schedtick：P 的调度次数
- syscalltick：P 的系统调用次数
- m：隶属哪一个 M
- runqsize：运行队列中的 G 数量
- gfreecnt：可用的G（状态为 Gdead）

P 的 5 种运行状态：

| 状态      | 值   | 含义                                                         |
| --------- | ---- | ------------------------------------------------------------ |
| _Pidle    | 0    | 刚刚被分配，还没有进行进行初始化。                           |
| _Prunning | 1    | 当 M 与 P 绑定调用 acquirep 时，P 的状态会改变为 _Prunning。 |
| _Psyscall | 2    | 正在执行系统调用。                                           |
| _Pgcstop  | 3    | 暂停运行，此时系统正在进行 GC，直至 GC 结束后才会转变到下一个状态阶段。 |
| _Pdead    | 4    | 废弃，不再使用。                                             |

