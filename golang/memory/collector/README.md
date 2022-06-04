# 垃圾回收 

## GC 原理
---
### GC 概述
垃圾回收也称为GC（Garbage Collection），是一种自动内存管理机制
现代高级编程语言管理内存的方式分为两种：自动和手动，像 C、C++ 等编程语言使用手动管理内存的方式，工程师编写代码过程中需要主动申请或者释放内存；而 PHP、Java 和 Go 等语言使用自动的内存管理系统，由`内存分配器`和`垃圾收集器`来代为分配和回收内存，其中垃圾收集器就是我们常说的 GC。

在应用程序中会使用到两种内存，分别为堆（Heap）和栈（Stack），GC 负责回收堆内存，而不负责回收栈中的内存：
- 栈是线程的专用内存，专门为了函数执行而准备的，存储着函数中的局部变量以及调用栈，函数执行完后，编译器可以将栈上分配的内存可以直接释放，不需要通过 GC 来回收。
- 堆是程序共享的内存，需要 GC 进行回收在堆上分配的内存。

垃圾回收器的执行过程被划分为两个半独立的组件：
- 赋值器（Mutator）：这一名称本质上是在指代用户态的代码。因为对垃圾回收器而言，用户态的代码仅仅只是在修改对象之间的引用关系，也就是在对象图（对象之间引用关系的一个有向图）上进行操作。
- 回收器（Collector）：负责执行垃圾回收的代码。

### 主流 GC 算法
目前比较常见的垃圾回收算法有三种：
1. 引用计数：为每个对象维护一个引用计数，当引用该对象的对象销毁时，引用计数 -1，当对象引用计数为 0 时回收该对象。
    - 代表语言：Python、PHP、Swift
    - 优点：对象回收快，不会出现内存耗尽或达到某个阈值时才回收。
    - 缺点：不能很好的处理循环引用，而实时维护引用计数也是有损耗的。

2. 分代收集：按照对象生命周期长短划分不同的代空间，生命周期长的放入老年代，短的放入新生代，不同代有不同的回收算法和回收频率。
    - 代表语言：Java
    - 优点：回收性能好
    - 缺点：算法复杂

3. 标记-清除：从根变量开始遍历所有引用的对象，标记引用的对象，没有被标记的进行回收。
    - 代表语言：Golang（三色标记法）
    - 优点：解决了引用计数的缺点。
    - 缺点：需要 STW，暂时停掉程序运行。

![标记-清除算法](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202205312152827.png)

### Go GC 算法
#### 三色标记法
此算法是在 Go 1.5 版本开始使用，Go 语言采用的是标记清除算法，并在此基础上使用了三色标记法和混合写屏障技术，GC 过程和其他用户 Goroutine 可并发运行，但需要一定时间的 STW。

三色标记法只是为了叙述方便而抽象出来的一种说法，实际上的对象是没有三色之分的。这里的三色，对应了垃圾回收过程中对象的三种状态：
- 灰色：对象还在标记队列中等待
- 黑色：对象已被标记，gcmarkBits 对应位为 1 （该对象不会在本次 GC 中被回收）
- 白色：对象未被标记，gcmarkBits 对应位为 0 （该对象将会在本次 GC 中被清理）

step 1: 创建白、灰、黑 三个集合
step 2: 将所有对象放入白色集合中
step 3: 遍历所有 root 对象，把遍历到的对象从白色集合放入灰色集合 (这里放入灰色集合的都是根节点的对象)
step 4: 遍历灰色集合，将灰色对象引用的对象从白色集合放入灰色集合，自身标记为黑色
step 5: 重复步骤4，直到灰色中无任何对象，其中用到2个机制：
- 写屏障（Write Barrier）：上面说到的 STW 的目的是防止 GC 扫描时内存变化引起的混乱，而写屏障就是让 Goroutine 与 GC 同时运行的手段，虽然不能完全消除 STW，但是可以大大减少 STW 的时间。写屏障在 GC 的特定时间开启，开启后指针传递时会把指针标记，即本轮不回收，下次 GC 时再确定。
- 辅助 GC（Mutator Assist）：为了防止内存分配过快，在 GC 执行过程中，GC 过程中 mutator 线程会并发运行，而 mutator assist 机制会协助 GC 做一部分的工作。
step 6: 收集所有白色对象（垃圾）

#### root 对象
根对象在垃圾回收的术语中又叫做根集合，它是垃圾回收器在标记过程时最先检查的对象，包括：
- 全局变量：程序在编译期就能确定的那些存在于程序整个生命周期的变量。 
- 执行栈：每个 Goroutine 都包含自己的执行栈，这些执行栈上指向堆内存的指针。 
- 寄存器：寄存器的值可能表示一个指针，参与计算的这些指针可能指向某些赋值器分配的堆内存区块。

#### 插入写屏障
对象被引用时触发的机制（只在堆内存中生效）：赋值器这一行为通知给并发执行的回收器，被引用的对象标记为灰色
缺点：结束时需要 STW 来重新扫描栈，标记栈上引用的白色对象的存活

#### 删除写屏障
对象被删除时触发的机制（只在堆内存中生效）：赋值器将这一行为通知给并发执行的回收器，被删除的对象，如果自身为灰色或者白色，那么标记为灰色
缺点：一个对象的引用被删除后，即使没有其他存活的对象引用它，它仍然会活到下一轮，会产生很大冗余扫描成本，且降低了回收精度

#### 混合写屏障
GC没有混合写屏障前，一直是插入写屏障；混合写屏障是插入写屏障 + 删除写屏障，写屏障只应用在堆上应用，栈上不启用（栈上启用成本很高）
- GC 开始将栈上的对象全部扫描并标记为黑色。
- GC 期间，任何在栈上创建的新对象，均为黑色。
- 被删除的对象标记为灰色。
- 被添加的对象标记为灰色。

### GC 流程
![三色标记法](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202205312152964.png)
一次完整的垃圾回收会分为四个阶段，分别是标记准备、标记开始、标记终止、清理：
- 标记准备（Mark Setup）：打开写屏障（Write Barrier），需 STW（stop the world)
- 标记开始（Marking）：使用三色标记法并发标记 ，与用户程序并发执行
- 标记终止（Mark Termination）：对触发写屏障的对象进行重新扫描标记，关闭写屏障（Write Barrier），需 STW（stop the world)
- 清理（Sweeping）：将需要回收的内存归还到堆中，将过多的内存归还给操作系统，与用户程序并发执行

### GC 触发时机
主动触发：
- 调用 runtime.GC() 方法，触发 GC。

被动触发：
- 定时触发，该触发条件由 runtime.forcegcperiod 变量控制，默认为 2 分钟。当超过两分钟没有产生任何 GC 时，触发 GC。
- 根据内存分配阈值触发，该触发条件由环境变量 GOGC 控制，默认值为 100（100%），当前堆内存占用是上次GC结束后占用内存的 2 倍时，触发 GC。

### GC 算法演进
- Go 1：mark and sweep 操作都需要 STW
- Go 1.3：分离了 mark 和 sweep 操作，mark 过程需要 STW，mark 完成后让 sweep 任务和普通协程任务一样并行，停顿时间在约几百ms
- Go 1.5：引入三色并发标记法、插入写屏障，不需要每次都扫描整个内存空间，可以减少 stop the world 的时间，停顿时间在 100ms 以内
- Go 1.6：使用 bitmap 来记录回收内存的位置，大幅优化垃圾回收器自身消耗的内存，停顿时间在 10ms 以内
- Go 1.7：停顿时间控制在 2ms 以内
- Go 1.8：混合写屏障（插入写屏障和删除写屏障），停顿时间在 0.5ms 左右
- Go 1.9：彻底移除了栈的重扫描过程
- Go 1.12：整合了两个阶段的 Mark Termination
- Go 1.13：着手解决向操作系统归还内存的，提出了新的 Scavenger
- Go 1.14：替代了仅存活了一个版本的 scavenger，全新的页分配器，优化分配内存过程的速率与现有的扩展性问题，并引入了异步抢占，解决了由于密集循环导致的 STW 时间过长的问题

## GC 调优
---
- 控制内存分配的速度，限制 Goroutine 的数量，提高赋值器 mutator 的 CPU 利用率（降低 GC 的 CPU 利用率）
- 少量使用 + 连接 string
- slice 提前分配足够的内存来降低扩容带来的拷贝
- 避免 map key 对象过多，导致扫描时间增加
- 变量复用，减少对象分配，例如使用 sync.Pool 来复用需要频繁创建临时对象、使用全局变量等
- 增大 GOGC 的值，降低 GC 的运行频率

## 查看 GC 信息
---
### GODEBUG=gctrace=1
```go
func main() {
    for n := 1; n < 100000; n++ {
        _ = make([]byte, 1<<20)
    }
}
```

查看 GC：
```shell
# GODEBUG=gctrace=1 go run main.go 
gc 1 @0.024s 0%: 0.017+0.43+0.015 ms clock, 0.14+0.27/0.31/0.16+0.12 ms cpu, 4->4->0 MB, 5 MB goal, 8 P
gc 2 @0.051s 0%: 0.14+0.84+0.004 ms clock, 1.1+0.12/0.42/0.031+0.037 ms cpu, 4->4->0 MB, 5 MB goal, 8 P
gc 3 @0.061s 0%: 0.18+0.46+0.022 ms clock, 1.5+0.15/0.56/0.023+0.18 ms cpu, 4->4->0 MB, 5 MB goal, 8 P
```

gctrace输出及各字段含义：
`gc # @#s #%: #+#+# ms clock, #+#/#/#+# ms cpu, #->#-># MB, # MB goal, # P`
- `gc #`: GC 序号，每次 GC 递增
- `@#s`: 程序启动后的时间
- `#%`: 程序启动后 GC 时间占比 
- `#+...+#`: GC 各阶段的时钟时间和 CPU 时间
- `#->#-># MB`: GC 开始时、结束时、活跃堆的堆大小
- `# MB goal`: 目标堆大小 
- `# P`: 已用处理器数量 


参考：https://pkg.go.dev/runtime#hdr-Environment_Variables

### go tool trace
```go
func main() {
    f, _ := os.Create("trace.out")
    defer f.Close()

    trace.Start(f)
    defer trace.Stop()

    for n := 1; n < 100000; n++ {
        _ = make([]byte, 1<<20)
    }
}
```

查看 GC：
```shell
# go run main.go

# go tool trace trace.out
2022/05/31 19:12:50 Parsing trace...
2022/05/31 19:12:54 Splitting trace...
2022/05/31 19:13:01 Opening browser. Trace viewer is listening on http://127.0.0.1:57376
```
浏览器打开 trace viewer 页面：
![trace viewer](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202205311925073.png)

查看 View trace： 
![View trace](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202205311925871.png)

查看 Minimum mutator utilization： 
![Minimum mutator utilization](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202205311926430.png)

### debug.ReadGCStats
```go
func printGCStats() {
    t := time.NewTicker(time.Second)
    s := debug.GCStats{}

    for {
        select {
        case <-t.C:
            debug.ReadGCStats(&s)
            fmt.Printf("gc %d last@%v, PauseTotal %v\n", s.NumGC, s.LastGC, s.PauseTotal)
        }
    }
}

func main() {
    go printGCStats()

    for n := 1; n < 100000; n++ {
        _ = make([]byte, 1<<20)
    }
}
```

查看 GC：
```shell
# go run main.go 
gc 2974 last@2022-05-31 19:30:23.322424 +0800 CST, PauseTotal 133.221849ms
gc 6012 last@2022-05-31 19:30:24.322509 +0800 CST, PauseTotal 275.603549ms
gc 8830 last@2022-05-31 19:30:25.32224 +0800 CST, PauseTotal 410.77667ms
```

GCStats 各字段含义：
```go
type GCStats struct {
    LastGC         time.Time       // time of last collection
    NumGC          int64           // number of garbage collections
    PauseTotal     time.Duration   // total pause for all collections
    Pause          []time.Duration // pause history, most recent first
    PauseEnd       []time.Time     // pause end times history, most recent first
    PauseQuantiles []time.Duration
}
```

### runtime.ReadMemStats
```go
func printMemStats() {
    t := time.NewTicker(time.Second)
    s := runtime.MemStats{}

    for {
        select {
        case <-t.C:
            runtime.ReadMemStats(&s)
            fmt.Printf("gc %d last@%v, heap_object_num: %v, heap_alloc: %vMB, next_heap_size: %vMB\n",
                s.NumGC, time.Unix(int64(time.Duration(s.LastGC).Seconds()), 0), s.HeapObjects, s.HeapAlloc/(1<<20), s.NextGC/(1<<20))
        }
    }
}

func main() {
    go printMemStats()

    for n := 1; n < 100000; n++ {
        _ = make([]byte, 1<<20)
    }
}
```

查看 GC：
```shell
$ go run main.go 
gc 2773 last@2022-05-31 19:34:27 +0800 CST, heap_object_num: 343, heap_alloc: 4MB, next_heap_size: 5MB
gc 5694 last@2022-05-31 19:34:28 +0800 CST, heap_object_num: 393, heap_alloc: 4MB, next_heap_size: 5MB
gc 8765 last@2022-05-31 19:34:29 +0800 CST, heap_object_num: 325, heap_alloc: 4MB, next_heap_size: 5MB
```

MemStats 各字段含义：
```go
type MemStats struct {

    // Alloc is bytes of allocated heap objects.
    //
    // This is the same as HeapAlloc (see below).
    Alloc uint64

    // TotalAlloc is cumulative bytes allocated for heap objects.
    //
    // TotalAlloc increases as heap objects are allocated, but
    // unlike Alloc and HeapAlloc, it does not decrease when
    // objects are freed.
    TotalAlloc uint64

    // Sys is the total bytes of memory obtained from the OS.
    //
    // Sys is the sum of the XSys fields below. Sys measures the
    // virtual address space reserved by the Go runtime for the
    // heap, stacks, and other internal data structures. It's
    // likely that not all of the virtual address space is backed
    // by physical memory at any given moment, though in general
    // it all was at some point.
    Sys uint64

    // Lookups is the number of pointer lookups performed by the
    // runtime.
    //
    // This is primarily useful for debugging runtime internals.
    Lookups uint64

    // Mallocs is the cumulative count of heap objects allocated.
    // The number of live objects is Mallocs - Frees.
    Mallocs uint64

    // Frees is the cumulative count of heap objects freed.
    Frees uint64

    // HeapAlloc is bytes of allocated heap objects.
    //
    // "Allocated" heap objects include all reachable objects, as
    // well as unreachable objects that the garbage collector has
    // not yet freed. Specifically, HeapAlloc increases as heap
    // objects are allocated and decreases as the heap is swept
    // and unreachable objects are freed. Sweeping occurs
    // incrementally between GC cycles, so these two processes
    // occur simultaneously, and as a result HeapAlloc tends to
    // change smoothly (in contrast with the sawtooth that is
    // typical of stop-the-world garbage collectors).
    HeapAlloc uint64

    // HeapSys is bytes of heap memory obtained from the OS.
    //
    // HeapSys measures the amount of virtual address space
    // reserved for the heap. This includes virtual address space
    // that has been reserved but not yet used, which consumes no
    // physical memory, but tends to be small, as well as virtual
    // address space for which the physical memory has been
    // returned to the OS after it became unused (see HeapReleased
    // for a measure of the latter).
    //
    // HeapSys estimates the largest size the heap has had.
    HeapSys uint64

    // HeapIdle is bytes in idle (unused) spans.
    //
    // Idle spans have no objects in them. These spans could be
    // (and may already have been) returned to the OS, or they can
    // be reused for heap allocations, or they can be reused as
    // stack memory.
    //
    // HeapIdle minus HeapReleased estimates the amount of memory
    // that could be returned to the OS, but is being retained by
    // the runtime so it can grow the heap without requesting more
    // memory from the OS. If this difference is significantly
    // larger than the heap size, it indicates there was a recent
    // transient spike in live heap size.
    HeapIdle uint64

    // HeapInuse is bytes in in-use spans.
    //
    // In-use spans have at least one object in them. These spans
    // can only be used for other objects of roughly the same
    // size.
    //
    // HeapInuse minus HeapAlloc estimates the amount of memory
    // that has been dedicated to particular size classes, but is
    // not currently being used. This is an upper bound on
    // fragmentation, but in general this memory can be reused
    // efficiently.
    HeapInuse uint64

    // HeapReleased is bytes of physical memory returned to the OS.
    //
    // This counts heap memory from idle spans that was returned
    // to the OS and has not yet been reacquired for the heap.
    HeapReleased uint64

    // HeapObjects is the number of allocated heap objects.
    //
    // Like HeapAlloc, this increases as objects are allocated and
    // decreases as the heap is swept and unreachable objects are
    // freed.
    HeapObjects uint64

    // StackInuse is bytes in stack spans.
    //
    // In-use stack spans have at least one stack in them. These
    // spans can only be used for other stacks of the same size.
    //
    // There is no StackIdle because unused stack spans are
    // returned to the heap (and hence counted toward HeapIdle).
    StackInuse uint64

    // StackSys is bytes of stack memory obtained from the OS.
    //
    // StackSys is StackInuse, plus any memory obtained directly
    // from the OS for OS thread stacks (which should be minimal).
    StackSys uint64

    // MSpanInuse is bytes of allocated mspan structures.
    MSpanInuse uint64

    // MSpanSys is bytes of memory obtained from the OS for mspan
    // structures.
    MSpanSys uint64

    // MCacheInuse is bytes of allocated mcache structures.
    MCacheInuse uint64

    // MCacheSys is bytes of memory obtained from the OS for
    // mcache structures.
    MCacheSys uint64

    // BuckHashSys is bytes of memory in profiling bucket hash tables.
    BuckHashSys uint64

    // GCSys is bytes of memory in garbage collection metadata.
    GCSys uint64

    // OtherSys is bytes of memory in miscellaneous off-heap
    // runtime allocations.
    OtherSys uint64

    // NextGC is the target heap size of the next GC cycle.
    //
    // The garbage collector's goal is to keep HeapAlloc ≤ NextGC.
    // At the end of each GC cycle, the target for the next cycle
    // is computed based on the amount of reachable data and the
    // value of GOGC.
    NextGC uint64

    // LastGC is the time the last garbage collection finished, as
    // nanoseconds since 1970 (the UNIX epoch).
    LastGC uint64

    // PauseTotalNs is the cumulative nanoseconds in GC
    // stop-the-world pauses since the program started.
    //
    // During a stop-the-world pause, all goroutines are paused
    // and only the garbage collector can run.
    PauseTotalNs uint64

    // PauseNs is a circular buffer of recent GC stop-the-world
    // pause times in nanoseconds.
    //
    // The most recent pause is at PauseNs[(NumGC+255)%256]. In
    // general, PauseNs[N%256] records the time paused in the most
    // recent N%256th GC cycle. There may be multiple pauses per
    // GC cycle; this is the sum of all pauses during a cycle.
    PauseNs [256]uint64

    // PauseEnd is a circular buffer of recent GC pause end times,
    // as nanoseconds since 1970 (the UNIX epoch).
    //
    // This buffer is filled the same way as PauseNs. There may be
    // multiple pauses per GC cycle; this records the end of the
    // last pause in a cycle.
    PauseEnd [256]uint64

    // NumGC is the number of completed GC cycles.
    NumGC uint32

    // NumForcedGC is the number of GC cycles that were forced by
    // the application calling the GC function.
    NumForcedGC uint32

    // GCCPUFraction is the fraction of this program's available
    // CPU time used by the GC since the program started.
    //
    // GCCPUFraction is expressed as a number between 0 and 1,
    // where 0 means GC has consumed none of this program's CPU. A
    // program's available CPU time is defined as the integral of
    // GOMAXPROCS since the program started. That is, if
    // GOMAXPROCS is 2 and a program has been running for 10
    // seconds, its "available CPU" is 20 seconds. GCCPUFraction
    // does not include CPU time used for write barrier activity.
    //
    // This is the same as the fraction of CPU reported by
    // GODEBUG=gctrace=1.
    GCCPUFraction float64

    // EnableGC indicates that GC is enabled. It is always true,
    // even if GOGC=off.
    EnableGC bool

    // DebugGC is currently unused.
    DebugGC bool

    // BySize reports per-size class allocation statistics.
    //
    // BySize[N] gives statistics for allocations of size S where
    // BySize[N-1].Size < S ≤ BySize[N].Size.
    //
    // This does not report allocations larger than BySize[60].Size.
    BySize [61]struct {
        // Size is the maximum byte size of an object in this
        // size class.
        Size uint32

        // Mallocs is the cumulative count of heap objects
        // allocated in this size class. The cumulative bytes
        // of allocation is Size*Mallocs. The number of live
        // objects in this size class is Mallocs - Frees.
        Mallocs uint64

        // Frees is the cumulative count of heap objects freed
        // in this size class.
        Frees uint64
    }
}
```
