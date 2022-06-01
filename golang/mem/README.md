# 内存分配

## 内存分配机制
Go 语言内置运行时（就是 Runtime），抛弃了传统的内存分配方式，改为自主管理。这样可以自主地实现更好的内存使用模式，比如内存池、预分配等等。这样，不会每次内存分配都需要进行系统调用。

### 设计思想
- 内存分配算法采用 Google 的 TCMalloc 算法，每个线程都会自行维护一个独立的内存池，进行内存分配时优先从该内存池中分配，当内存池不足时才会加锁向全局内存池申请，减少系统调用并且避免不同线程对全局内存池的锁竞争。
- 把内存切分的非常的细小，分为多级管理，以降低锁的粒度。
- 回收对象内存时，并没有将其真正释放掉，只是放回预先分配的大块内存中，以便复用。只有内存闲置过多的时候，才会尝试归还部分内存给操作系统，降低整体开销。

### 分配组件
Go 的内存管理组件主要有：mspan、mcache、mcentral 和 mheap。
![内存分配组件](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206010113784.png)

#### 内存管理单元 mspan
mspan 是内存管理的基本单元，该结构体中包含 next 和 prev 两个字段，它们分别指向了前一个和后一个 mspan，每个 mspan 都管理 npages 个大小为 8KB 的页，一个 span 是由多个 page 组成的，这里的页不是操作系统中的内存页，它们是操作系统内存页的整数倍。
page 是内存存储的基本单元，“对象”放到 page 中。
```go
type mspan struct {
    next        *mspan      // 后指针
    prev        *mspan      // 前指针
    startAddr   uintptr     // 管理页的起始地址，指向 page 
    npages      uintptr     // span 中 page 数量
    spanclass   spanClass   // 规格 
    // ...
}

type spanClass uint8
```

Go 有 68 种不同大小的 spanClass，用于小对象的分配。
```go
const _NumSizeClasses = 68
var class_to_size = [_NumSizeClasses]uint16{0, 8, 16, 24, 32, 48, 64, 80, 96, 112, 128, 144, 160, 176, 192, 208, 224, 240, 256, 288, 320, 352, 384, 416, 448, 480, 512, 576, 640, 704, 768, 896, 1024, 1152, 1280, 1408, 1536, 1792, 2048, 2304, 2688, 3072, 3200, 3456, 4096, 4864, 5376, 6144, 6528, 6784, 6912, 8192, 9472, 9728, 10240, 10880, 12288, 13568, 14336, 16384, 18432, 19072, 20480, 21760, 24576, 27264, 28672, 32768}
```
如果按照序号为 1 的 spanClass（对象规格为 8B）分配，每个 span 占用堆的字节数：8k，mspan 可以保存 1024 个对象；
如果按照序号为 2 的 spanClass（对象规格为 16B）分配，每个 span 占用堆的字节数：8k，mspan 可以保存 512 个对象；
...
如果按照序号为 67 的 spanClass（对象规格为 32K）分配，每个 span 占用堆的字节数：32k，mspan 可以保存 1 个对象。

| class | bytes/obj | bytes/span | objects | tail waste | max waste |
| :---: | :-------: | :--------: | :-----: | :--------: | :-------: |
|   1   |     8     |    8192    |  1024   |     0      |  87.50%   |
|   2   |    16     |    8192    |   512   |     0      |  43.75%   |
|  ...  |    ...    |    ...     |   ...   |    ...     |    ...    |
|  67   |   32768   |   32768    |    1    |     0      |  12.50%   |

各字段含义：
- `class`：class ID，每个 span 结构中都有一个 class ID, 表示该 span 可处理的对象类型
- `bytes/obj`：该 class 代表对象的字节数
- `bytes/span`：每个 span 占用堆的字节数，也即页数*页大小
- `objects`：每个 span 可分配的对象个数，也即（bytes/spans）/（bytes/obj）
- `waste bytes`：每个 span 产生的内存碎片，也即（bytes/spans）%（bytes/obj）

大于 32k 的对象出现时，会直接从 heap 分配一个特殊的 span，这个特殊的 span 的类型（class）是 0, 只包含了一个大对象。

#### 线程缓存 mcache
mcache 管理线程在本地缓存的 mspan，每个 goroutine 绑定的 P 都有一个 mcache 字段。
```go
type mcache struct {
    alloc [numSpanClasses]*mspan
}

_NumSizeClasses = 68
numSpanClasses = _NumSizeClasses << 1
```

mcache 用 Span Classes 作为索引管理多个用于分配的 mspan，它包含所有规格的 mspan。它是 _NumSizeClasses 的 2 倍，也就是 68*2=136，其中 *2 是将 spanClass 分成了有指针和没有指针两种，方便与垃圾回收。对于每种规格，有 2 个 mspan，一个 mspan 不包含指针，另一个 mspan 则包含指针。对于无指针对象的 mspan 在进行垃圾回收的时候无需进一步扫描它是否引用了其他活跃的对象。

mcache 在初始化的时候是没有任何 mspan 资源的，在使用过程中会动态地从 mcentral 申请，之后会缓存下来。当对象小于等于 32KB 大小时，使用 mcache 的相应规格的 mspan 进行分配。

#### 中心缓存 mcentral
mcentral 管理全局的 mspan 供所有线程使用，全局 mheap 变量包含 central 字段，每个 mcentral 结构都维护在 mheap 结构内。
```go
type mcentral struct {
    spanclass spanClass     // 指当前规格大小
    partial   [2]spanSet    // 有空闲object的mspan列表
    full      [2]spanSet    // 没有空闲object的mspan列表
}
```

每个 mcentral 管理一种 spanClass 的 mspan，并将有空闲空间和没有空闲空间的 mspan 分开管理。partial 和 full 的数据类型为 spanSet，表示 mspans 集，可以通过 pop、push 来获得 mspans。
```go
type spanSet struct {
    spineLock mutex
    spine     unsafe.Pointer // 指向[]span的指针
    spineLen  uintptr        // Spine array length, accessed atomically
    spineCap  uintptr        // Spine array cap, accessed under lock
    index     headTailIndex  // 前32位是头指针，后32位是尾指针
}
```

简单说下 mcache 从 mcentral 获取和归还 mspan 的流程：
- 获取：加锁，从 partial 链表找到一个可用的 mspan；并将其从 partial 链表删除；将取出的 mspan 加入到 full 链表；将 mspan 返回给工作线程，解锁。
- 归还：加锁，将 mspan 从 full 链表删除；将 mspan 加入到 partial 链表，解锁。

#### 页堆 mheap
mheap 管理 Go 的所有动态分配内存，可以认为是 Go 程序持有的整个堆空间，全局唯一。
```go
var mheap_ mheap
type mheap struct {
    lock      mutex     // 全局锁
    pages     pageAlloc // 页面分配的数据结构
    allspans []*mspan   // 所有通过 mheap_ 申请的 mspans
    // 堆
    arenas [1 << arenaL1Bits]*[1 << arenaL2Bits]*heapArena
    
    // 所有中心缓存 mcentral
    central [numSpanClasses]struct {
        mcentral mcentral
        pad      [cpu.CacheLinePadSize - unsafe.Sizeof(mcentral{})%cpu.CacheLinePadSize]byte
    }
    ...
}
```

所有 mcentral 的集合则是存放于 mheap 中的。mheap 里的 arena 区域是堆内存的抽象，运行时会将 8KB 看做一页，这些内存页中存储了所有在堆上初始化的对象。运行时使用二维的 runtime.heapArena 数组管理所有的内存，每个 runtime.heapArena 都会管理 64MB 的内存。

当申请内存时，依次经过 mcache 和 mcentral 都没有可用合适规格的大小内存，这时候会向 mheap 申请一块内存。然后按指定规格划分为一些列表，并将其添加到相同规格大小的 mcentral 的非空闲列表后面。

### 分配对象
- 微对象 (0, 16B)：先使用线程缓存上的微型分配器，再依次尝试线程缓存、中心缓存、堆 分配内存。
- 小对象 [16B, 32KB]：依次尝试线程缓存、中心缓存、堆 分配内存。
- 大对象 (32KB, +∞)：直接尝试堆分配内存。

### 分配流程
![内存分配流程](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206010007976.png)

1. 首先通过计算使用的大小规格。
2. 然后使用 mcache 中对应大小规格的块分配。
3. 如果 mcentral 中没有可用的块，则向 mheap 申请，并根据算法找到最合适的 mspan。
4. 如果申请到的 mspan 超出申请大小，将会根据需求进行切分，以返回用户所需的页数。剩余的页构成一个新的 mspan 放回 mheap 的空闲列表。
5. 如果 mheap 中没有可用 span，则向操作系统申请一系列新的页（最小 1MB）

## 内存逃逸机制

### 概念
在一段程序中，每一个函数都会有自己的内存区域存放自己的局部变量、返回地址等，这些内存会由编译器在栈中进行分配，每一个函数都会分配一个栈桢，在函数运行结束后进行销毁，但是有些变量我们想在函数运行结束后仍然使用它，那么就需要把这个变量在堆上分配，这种从“栈”上逃逸到“堆”上的现象就成为内存逃逸。

在栈上分配的地址，一般由系统申请和释放，不会有额外性能的开销，比如函数的入参、局部变量、返回值等。
在堆上分配的内存，如果要回收掉，需要进行 GC，那么 GC 一定会带来额外的性能开销。
编程语言不断优化 GC 算法，主要目的都是为了减少 GC 带来的额外性能开销，变量一旦逃逸会导致性能开销变大。

### 逃逸机制
编译器会根据变量是否被外部引用来决定是否逃逸：
1. 如果函数外部没有引用，则优先放到栈中
2. 如果函数外部存在引用，则必定放到堆中
3. 如果栈上放不下，则必定放到堆上

逃逸分析也就是由编译器决定哪些变量放在栈，哪些放在堆中，通过编译参数 -gcflag=-m 可以查看编译过程中的逃逸分析，发生逃逸的几种场景如下：

#### 指针逃逸
```go
func escape() {
    s := make([]int, 0, 10000)

    for index := range s {
        s[index] = index
    }
}

func main() {
    escape()
}
```
查看逃逸情况：
```shell
# go build -gcflags=-m main.go
# command-line-arguments
./main.go:4:6: can inline escape
./main.go:9:6: can inline main
./main.go:10:8: inlining call to escape
./main.go:5:6: moved to heap: a
```
函数返回值为局部变量的指针，函数虽然退出了，但是因为指针的存在，指向的内存不能随着函数结束而回收，因此只能分配在堆上。

#### 栈空间不足
```go
func escape() {
    s := make([]int, 0, 10000)

    for index := range s {
        s[index] = index
    }
}

func main() {
    escape()
}
```
查看逃逸情况：
```shell
# go build -gcflags=-m main.go
# command-line-arguments
./main.go:12:6: can inline main
./main.go:5:11: make([]int, 0, 10000) escapes to heap
```
当栈空间足够时，不会发生逃逸，但是当变量过大时，已经完全超过栈空间的大小时，将会发生逃逸到堆上分配内存。局部变量 s 占用内存过大，编译器会将其分配到堆上。

#### 变量大小不确定
```go
func escape() {
    number := 10
    s := make([]int, number) // 编译期间无法确定slice的长度

    for i := 0; i < len(s); i++ {
        s[i] = i
    }
}

func main() {
    escape()
}
```
查看逃逸情况：
```shell
# go build -gcflags=-m main.go
# command-line-arguments
./main.go:4:6: can inline escape
./main.go:13:6: can inline main
./main.go:14:8: inlining call to escape
./main.go:6:11: make([]int, number) escapes to heap
./main.go:14:8: make([]int, number) escapes to heap
```
编译期间无法确定 slice 的长度，这种情况为了保证内存的安全，编译器也会触发逃逸，在堆上进行分配内存。直接 s := make([]int, 10) 不会发生逃逸。

#### 动态类型
动态类型就是编译期间不确定参数的类型、参数的长度也不确定的情况下就会发生逃逸。
空接口 interface{} 可以表示任意的类型，如果函数参数为 interface{}，编译期间很难确定其参数的具体类型，也会发生逃逸。

```go
func escape() {
    fmt.Println(1111)
}

func main() {
    escape()
}
```
查看逃逸情况：
```shell
# go build -gcflags=-m main.go
# command-line-arguments
./main.go:6:6: can inline escape4
./main.go:7:13: inlining call to fmt.Println
./main.go:10:6: can inline main
./main.go:11:9: inlining call to escape4
./main.go:11:9: inlining call to fmt.Println
./main.go:7:14: 1111 escapes to heap
./main.go:7:13: []interface {}{...} does not escape
./main.go:11:9: 1111 escapes to heap
./main.go:11:9: []interface {}{...} does not escape
<autogenerated>:1: leaking param content: .this
```
fmt.Println(a ...interface{}) 函数参数为 interface，编译器不确定参数的类型，会将变量分配到堆上。

#### 闭包引用对象
```go
func escape() func() int {
    var i int = 1

    return func() int {
        i++
        return i
    }
}

func main() {
    escape()
}
```
查看逃逸情况：
```shell
# go build -gcflags=-m main.go
# command-line-arguments
./main.go:4:6: can inline escape5
./main.go:7:9: can inline escape5.func1
./main.go:13:6: can inline main
./main.go:14:9: inlining call to escape5
./main.go:5:6: moved to heap: i
./main.go:7:9: func literal escapes to heap
./main.go:7:9: func literal does not escape
```

### 总结
1. 栈上分配内存比在堆中分配内存效率更高
2. 栈上分配的内存不需要 GC 处理，而堆需要
3. 逃逸分析目的是决定内分配地址是栈还是堆
4. 逃逸分析在编译阶段完成
因为无论变量的大小，只要是指针变量都会在堆上分配，所以对于小变量我们还是使用传值效率（而不是传指针）更高一点。

## 内存对齐机制

### 内存对齐概述
为了能让 CPU 可以更快的存取到各个字段，Go编译器会帮你把 struct 结构体做数据的对齐。
所谓的数据对齐，是指内存地址是所存储数据大小（按字节为单位）的整数倍，以便 CPU 可以一次将该数据从内存中读取出来。
编译器通过在结构体的各个字段之间填充一些空白已达到对齐的目的。

### 对齐系数
不同硬件平台占用的大小和对齐值都可能是不一样的，每个特定平台上的编译器都有自己的默认“对齐系数”，32 位系统对齐系数是 4，64 位系统对齐系数是 8。
不同类型的对齐系数也可能不一样，使用Go语言中的 unsafe.Alignof 函数可以返回相应类型的对齐系数，对齐系数都符合 2^n 这个规律，最大也不会超过 8。

查看各类型在 MacOS 64 系统上的对齐系数：
```go
func main() {
    fmt.Printf("bool alignof is %d\n", unsafe.Alignof(bool(true)))    // 1
    fmt.Printf("string alignof is %d\n", unsafe.Alignof(string("a"))) // 8
    fmt.Printf("int8 alignof is %d\n", unsafe.Alignof(int8(0)))       // 1
    fmt.Printf("int16 alignof is %d\n", unsafe.Alignof(int16(0)))     // 2
    fmt.Printf("int32 alignof is %d\n", unsafe.Alignof(int32(0)))     // 4
    fmt.Printf("int64 alignof is %d\n", unsafe.Alignof(int64(0)))     // 8
    fmt.Printf("int alignof is %d\n", unsafe.Alignof(int(0)))         // 8
    fmt.Printf("float32 alignof is %d\n", unsafe.Alignof(float32(0))) // 4
    fmt.Printf("float64 alignof is %d\n", unsafe.Alignof(float64(0))) // 8
}
```

内存对齐的优缺点：
- 优点
    - 提高可移植性，有些 CPU 可以访问任意地址上的任意数据，而有些 CPU 只能在特定地址访问数据，因此不同硬件平台具有差异性，这样的代码就不具有移植性，如果在编译时，将分配的内存进行对齐，这就具有平台可以移植性了。
    - 提高内存的访问效率，32 位 CPU 下一次可以从内存中读取 32 位（4个字节）的数据，64 位 CPU 下一次可以从内存中读取 64 位（8个字节）的数据，这个长度也称为 CPU 的字长。CPU一次可以读取 1 个字长的数据到内存中，如果所需要读取的数据正好跨了 1 个字长，那就得花两个 CPU 周期的时间去读取了。因此在内存中存放数据时进行对齐，可以提高内存访问效率。
- 缺点
    - 存在内存空间的浪费，实际上是空间换时间。

### 结构体对齐
结构体对齐原则：
- 结构体变量中成员的偏移量必须是成员大小的整数倍
- 整个结构体的地址必须是最大字节的整数倍（结构体的内存占用是1/4/8/16byte...)

```go
type T1 struct {
    i16  int16 // 2 byte
    bool bool  // 1 byte
}

type T2 struct {
    i8  int8  // 1 byte
    i64 int64 // 8 byte
    i32 int32 // 4 byte
}

type T3 struct {
    i8  int8  // 1 byte
    i32 int32 // 4 byte
    i64 int64 // 8 byte
}

func main() {
    fmt.Println(runtime.GOARCH) // amd64

    t1 := T1{}
    fmt.Println(unsafe.Sizeof(t1)) // 4 bytes

    t2 := T2{}
    fmt.Println(unsafe.Sizeof(t2)) // 24 bytes

    t3 := T3{}
    fmt.Println(unsafe.Sizeof(t3)) // 16 bytes
}
```

结果分析：
![结构体内存对齐](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202205312243692.png)
