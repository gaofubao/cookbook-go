# Mutex

## 互斥锁实现原理
---
Go sync 包提供了两种锁类型：互斥锁 sync.Mutex 和 读写互斥锁 sync.RWMutex，都属于悲观锁。

### 概述
Mutex 是互斥锁，当一个 goroutine 获得了锁后，其他 goroutine 不能获取锁（只能存在一个写或读，不能同时读和写）。

### 使用场景
多个线程同时访问临界区，为保证数据的安全，需锁住一些共享资源，防止并发访问这些共享数据时可能导致的数据不一致问题。
获取锁的线程可以正常访问临界区，未获取到锁的线程等待锁释放后可以尝试获取锁。

![锁](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041753560.png)

### 底层实现
互斥锁对应的是底层结构是 sync.Mutex 结构体，位于 src/sync/mutex.go 中：
```go
type Mutex struct {
    state int32
    sema  uint32
}
```
- state 表示锁的状态，有锁定、被唤醒、饥饿模式等，并且是用 state 的二进制位来标识的，不同模式下会有不同的处理方式。
- sema 表示信号量，mutex 阻塞队列的定位是通过这个变量来实现的，从而实现 goroutine 的阻塞和唤醒。

```go
addr = &sema
func semroot(addr *uint32) *semaRoot {  
   return &semtable[(uintptr(unsafe.Pointer(addr))>>3)%semTabSize].root  
}
root := semroot(addr)
root.queue(addr, s, lifo)
root.dequeue(addr)

var semtable [251]struct {  
    root semaRoot  
    // ...
}

type semaRoot struct {  
    lock  mutex  
    treap *sudog // root of balanced tree of unique waiters.  
    nwait uint32 // Number of waiters. Read w/o the lock.  
}

type sudog struct {
    g *g  
    next *sudog  
    prev *sudog
    elem unsafe.Pointer // 指向sema变量
    waitlink *sudog     // g.waiting list or semaRoot  
    waittail *sudog     // semaRoot
    // ...
}
```

![mutex](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041911876.png)

### 操作
锁的实现一般会依赖于原子操作、信号量，通过 atomic 包中的一些原子操作来实现锁的锁定，通过信号量来实现线程的阻塞与唤醒。

#### 加锁
通过原子操作 cas 加锁，如果加锁不成功，根据不同的场景选择自旋重试加锁或者阻塞等待被唤醒后加锁。

![加锁](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041912795.png)

```go
func (m *Mutex) Lock() {
    // Fast path: 幸运之路，一下就获取到了锁
    if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
        return
    }
    // Slow path：缓慢之路，尝试自旋或阻塞获取锁
    m.lockSlow()
}
```

#### 解锁
通过原子操作 add 解锁，如果仍有 goroutine 在等待，唤醒等待的 goroutine。

![解锁](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206041912818.png)

```go
func (m *Mutex) Unlock() {  
    // Fast path: 幸运之路，解锁
    new := atomic.AddInt32(&m.state, -mutexLocked)  
    if new != 0 {  
        // Slow path：如果有等待的goroutine，唤醒等待的goroutine
        m.unlockSlow()
    }  
}
```

#### 注意事项
- 在 Lock() 之前使用 Unlock() 会导致 panic 异常。
- 使用 Lock() 加锁后，再次 Lock() 会导致死锁（不支持重入），需Unlock()解锁后才能再加锁。
- 锁定状态与 goroutine 没有关联，一个 goroutine 可以 Lock，另一个 goroutine 可以 Unlock。

## 互斥锁正常模式和饥饿模式
---
在 Go 中一共可以分为两种抢锁的模式，一种是正常模式，另外一种是饥饿模式。

### 正常模式(非公平锁)
在刚开始的时候，是处于正常模式（Barging），也就是，当一个 G1 持有着一个锁的时候，G2 会自旋的去尝试获取这个锁。
当自旋超过 4 次还没有能获取到锁的时候，这个 G2 就会被加入到获取锁的等待队列里面，并阻塞等待唤醒。
正常模式下，所有等待锁的 goroutine 按照 FIFO（先进先出）顺序等待。唤醒的 goroutine 不会直接拥有锁，而是会和新请求锁的 goroutine 竞争锁。新请求锁的 goroutine 具有优势：它正在 CPU 上执行，而且可能有好几个，所以刚刚唤醒的 goroutine 有很大可能在锁竞争中失败，长时间获取不到锁，就会切换到饥饿模式。

### 饥饿模式(公平锁)
当一个 goroutine 等待锁时间超过 1 毫秒时，它可能会遇到饥饿问题。在版本 1.9 中，这种场景下 Go Mutex 切换到饥饿模式（handoff），解决饥饿问题。
```go
starving = runtime_nanotime() - waitStartTime > 1e6
```

但是也不可能永远地保持在饥饿状态，总归会有吃饱的时候，也就是总有那么一刻 Mutex 会回归到正常模式，那么回归正常模式必须具备的条件有以下几种：
1. G 的执行时间小于 1ms
2. 等待队列已经全部清空了

当满足上述两个条件中的任意一个，Mutex 就会切换回正常模式，而 Go 的抢锁过程，就是在这个正常模式和饥饿模式中来回切换进行的。
```go
delta := int32(mutexLocked - 1<<mutexWaiterShift)  
if !starving || old>>mutexWaiterShift == 1 {  
    delta -= mutexStarving
}
atomic.AddInt32(&m.state, delta)
```

### 总结
对于两种模式，正常模式下的性能是最好的，goroutine 可以连续多次获取锁，饥饿模式解决了取锁公平的问题，但是性能会下降，其实是性能和公平的一个平衡模式。

## 互斥锁允许自旋的条件
线程没有获取到锁时有两种常见的处理方式：
- 一种是没有获取到锁的线程就一直循环等待判断该资源是否已经释放锁，这种锁也叫做自旋锁，它不用将线程阻塞起来，适用于并发低且程序执行时间短的场景，缺点是 CPU 占用较高。
- 另外一种处理方式就是把自己阻塞起来，释放 CPU 给其他线程，内核会将线程置为睡眠状态，等到锁被释放后，内核会在合适的时机唤醒该线程，适用于高并发场景，缺点是有线程上下文切换的开销。

Go 语言中的 Mutex 实现了自旋与阻塞两种场景，当满足不了自旋条件时，就会进入阻塞。
允许自旋的条件：
1. 锁已被占用，并且锁不处于饥饿模式。
2. 积累的自旋次数小于最大自旋次数（active_spin=4）。
3. CPU 核数大于 1。
4. 有空闲的 P。
5. 当前 goroutine 所挂载的 P 下，本地待运行队列为空。

```go
if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {  
    // ...
    runtime_doSpin()
    continue
}

func sync_runtime_canSpin(i int) bool {  
    if i >= active_spin 
    || ncpu <= 1 
    || gomaxprocs <= int32(sched.npidle+sched.nmspinning)+1 {
        return false  
    }
    if p := getg().m.p.ptr(); !runqempty(p) {  
        return false  
    }
    return true
}

// 自旋
func sync_runtime_doSpin() {
    procyield(active_spin_cnt)
}
```
如果可以进入自旋状态之后就会调用 runtime_doSpin 方法进入自旋，doSpin 方法会调用 procyield(30) 执行 30 次 PAUSE 指令，什么都不做，但是会消耗CPU时间。

## 读写锁实现原理
---

### 概述
读写互斥锁 RWMutex，是对 Mutex 的一个扩展，当一个 goroutine 获得了读锁后，其他 goroutine 可以获取读锁，但不能获取写锁；当一个 goroutine 获得了写锁后，其他 goroutine 既不能获取读锁也不能获取写锁（只能存在一个写或多个读，可以同时读）。

### 使用场景
读多于写的情况（既保证线程安全，又保证性能不太差）。

### 底层实现
互斥锁对应的是底层结构是 sync.RWMutex 结构体，位于 src/sync/rwmutex.go 中。

```go
type RWMutex struct {
    w           Mutex  // 复用互斥锁
    writerSem   uint32 // 信号量，用于写等待读
    readerSem   uint32 // 信号量，用于读等待写
    readerCount int32  // 当前执行读的 goroutine 数量
    readerWait  int32  // 被阻塞的准备读的 goroutine 的数量
}
```

### 操作
读锁的加锁与释放：
```go
func (rw *RWMutex) RLock()      // 加读锁
func (rw *RWMutex) RUnlock()    // 释放读锁
```

#### 加读锁
```go
func (rw *RWMutex) RLock() {
    // 为什么readerCount会小于0呢？
    // 往下看发现writer的Lock()会对readerCount做减法操作（原子操作）
    if atomic.AddInt32(&rw.readerCount, 1) < 0 {
        // A writer is pending, wait for it.
        runtime_Semacquire(&rw.readerSem)
    }
}
```
atomic.AddInt32(&rw.readerCount, 1) 调用这个原子方法，对当前在读的数量加 1，如果返回负数，那么说明当前有其他写锁，这时候就调用 runtime_SemacquireMutex 休眠当前 goroutine 等待被唤醒。

#### 释放读锁
解锁的时候对正在读操作减 1，如果返回值小于 0，那么说明当前有写操作，这个时候调用 rUnlockSlow 进入慢速通道。
```go
func (rw *RWMutex) RUnlock() {
    if r := atomic.AddInt32(&rw.readerCount, -1); r < 0 {
        rw.rUnlockSlow(r)
    }
}
```
被阻塞的准备读的 goroutine 的数量减 1，readerWait 为 0，就表示当前没有正在准备读的  goroutine，这时候调用 runtime_Semrelease 唤醒写操作。
```go
func (rw *RWMutex) rUnlockSlow(r int32) {
    // A writer is pending.
    if atomic.AddInt32(&rw.readerWait, -1) == 0 {
        // The last reader unblocks the writer.
        runtime_Semrelease(&rw.writerSem, false, 1)
    }
}
```

写锁的加锁与释放：
```go
func (rw *RWMutex) Lock()   // 加写锁
func (rw *RWMutex) Unlock() // 释放写锁
```

#### 加写锁
```go
const rwmutexMaxReaders = 1 << 30

func (rw *RWMutex) Lock() {
    // First, resolve competition with other writers.
    rw.w.Lock()
    // Announce to readers there is a pending writer.
    r := atomic.AddInt32(&rw.readerCount, -rwmutexMaxReaders) + rwmutexMaxReaders
    // Wait for active readers.
    if r != 0 && atomic.AddInt32(&rw.readerWait, r) != 0 {
        runtime_Semacquire(&rw.writerSem)
    }
}
```

首先调用互斥锁的 lock，获取到互斥锁之后，如果计算之后当前仍然有其他 goroutine 持有读锁，那么就调用 runtime_SemacquireMutex 休眠当前的 goroutine 等待所有的读操作完成。
这里 readerCount 原子性加上一个很大的负数，是防止后面的协程能拿到读锁，阻塞读。

#### 释放写锁
```go
func (rw *RWMutex) Unlock() {
    // Announce to readers there is no active writer.
    r := atomic.AddInt32(&rw.readerCount, rwmutexMaxReaders)
    // Unblock blocked readers, if any.
    for i := 0; i < int(r); i++ {
        runtime_Semrelease(&rw.readerSem, false)
    }
    // Allow other writers to proceed.
    rw.w.Unlock()
}
```

解锁的操作，会先调用 atomic.AddInt32(&rw.readerCount, rwmutexMaxReaders) 将恢复之前写入的负数，然后根据当前有多少个读操作在等待，循环唤醒。

#### 注意点
- 读锁或写锁在 Lock() 之前使用 Unlock() 会导致 panic 异常。
- 使用 Lock() 加锁后，再次 Lock() 会导致死锁（不支持重入），需 Unlock() 解锁后才能再加锁。
- 锁定状态与 goroutine 没有关联，一个 goroutine 可以 RLock（Lock），另一个 goroutine 可以 RUnlock（Unlock）。

### 互斥锁和读写锁的区别
- 读写锁区分读和写，而互斥锁不区分。
- 互斥锁同一时间只允许一个线程访问该对象，无论读写；读写锁同一时间内只允许一个写者，但是允许多个读者同时读对象。

## 实现可重入锁
可重入锁又称为递归锁，是指在同一个线程在外层方法获取锁的时候，在进入该线程的内层方法时会自动获取锁，不会因为之前已经获取过还没释放再次加锁导致死锁。

为什么 Go 语言中没有可重入锁？
Mutex 不是可重入的锁。Mutex 的实现中没有记录哪个 goroutine 拥有这把锁。理论上，任何 goroutine 都可以随意地 Unlock 这把锁，所以没办法计算重入条件，并且 Mutex 重复 Lock 会导致死锁。

实现一个可重入锁需要这两点：
- 记住持有锁的线程
- 统计重入的次数

```go
func main() {
    var mutex = &ReentrantMutex{}
    mutex.Lock()
    mutex.Lock()
    fmt.Println(111)
    mutex.Unlock()
    mutex.Unlock()
}

type ReentrantLock struct {
    sync.Mutex
    recursion int32 // 这个goroutine 重入的次数
    owner     int64 // 当前持有锁的goroutine id
}

// Get returns the id of the current goroutine.
func GetGoroutineID() int64 {
    var buf [64]byte
    var s = buf[:runtime.Stack(buf[:], false)]
    s = s[len("goroutine "):]
    s = s[:bytes.IndexByte(s, ' ')]
    gid, _ := strconv.ParseInt(string(s), 10, 64)
    return gid
}

func NewReentrantLock() sync.Locker {
    res := &ReentrantLock{
        Mutex:     sync.Mutex{},
        recursion: 0,
        owner:     0,
    }
    return res
}

// ReentrantMutex 包装一个Mutex,实现可重入
type ReentrantMutex struct {
    sync.Mutex
    owner     int64 // 当前持有锁的goroutine id
    recursion int32 // 这个goroutine 重入的次数
}

func (m *ReentrantMutex) Lock() {
    gid := GetGoroutineID()
    // 如果当前持有锁的goroutine就是这次调用的goroutine,说明是重入
    if atomic.LoadInt64(&m.owner) == gid {
        m.recursion++
        return
    }
    m.Mutex.Lock()
    // 获得锁的goroutine第一次调用，记录下它的goroutine id,调用次数加1
    atomic.StoreInt64(&m.owner, gid)
    m.recursion = 1
}

func (m *ReentrantMutex) Unlock() {
    gid := GetGoroutineID()
    // 非持有锁的goroutine尝试释放锁，错误的使用
    if atomic.LoadInt64(&m.owner) != gid {
        panic(fmt.Sprintf("wrong the owner(%d): %d!", m.owner, gid))
    }
    // 调用次数减1
    m.recursion--
    if m.recursion != 0 { // 如果这个goroutine还没有完全释放，则直接返回
        return
    }
    // 此goroutine最后一次调用，需要释放锁
    atomic.StoreInt64(&m.owner, -1)
    m.Mutex.Unlock()
}
```

## Go 原子操作
Go atomic 包是最轻量级的锁（也称无锁结构），可以在不形成临界区和创建互斥量的情况下完成并发安全的值替换操作，不过这个包只支持 int32/int64/uint32/uint64/uintptr 这几种数据类型的一些基础操作（增减、交换、载入、存储等）。

### 概述
原子操作仅会由一个独立的 CPU 指令代表和完成。原子操作是无锁的，常常直接通过 CPU 指令直接实现。事实上，其它同步技术的实现常常依赖于原子操作。

### 使用场景
当我们想要对某个变量并发安全的修改，除了使用官方提供的 mutex，还可以使用 sync/atomic 包的原子操作，它能够保证对变量的读取或修改期间不被其他的协程所影响。
atomic 包提供的原子操作能够确保任一时刻只有一个 goroutine 对变量进行操作，善用 atomic 能够避免程序中出现大量的锁操作。

### 常见操作
- 增减 Add
- 载入 Load
- 比较并交换 CompareAndSwap
- 交换 Swap
- 存储 Store

atomic 操作的对象是一个地址，你需要把可寻址的变量的地址作为参数传递给方法，而不是把变量的值传递给方法。

#### 增减操作
此类操作的前缀为 Add：
```go
func AddInt32(addr *int32, delta int32) (new int32)

func AddInt64(addr *int64, delta int64) (new int64)

func AddUint32(addr *uint32, delta uint32) (new uint32)

func AddUint64(addr *uint64, delta uint64) (new uint64)

func AddUintptr(addr *uintptr, delta uintptr) (new uintptr)
```
需要注意的是，第一个参数必须是指针类型的值，通过指针变量可以获取被操作数在内存中的地址，从而施加特殊的 CPU 指令，确保同一时间只有一个 goroutine 能够进行操作。

使用举例：
```go
func add(addr *int64, delta int64) {
    atomic.AddInt64(addr, delta) //加操作
    fmt.Println("add opts: ", *addr)
}
```

#### 载入操作
此类操作的前缀为 Load：
```go
func LoadInt32(addr *int32) (val int32)

func LoadInt64(addr *int64) (val int64)

func LoadPointer(addr *unsafe.Pointer) (val unsafe.Pointer)

func LoadUint32(addr *uint32) (val uint32)

func LoadUint64(addr *uint64) (val uint64)

func LoadUintptr(addr *uintptr) (val uintptr)

// 特殊类型：Value类型，常用于配置变更
func (v *Value) Load() (x interface{}) {}
```
载入操作能够保证原子的读变量的值，当读取的时候，任何其他 CPU 操作都无法对该变量进行读写，其实现机制受到底层硬件的支持。

使用示例:
```go
func load(addr *int64) {
    fmt.Println("load opts: ", atomic.LoadInt64(&opts))
}
```

#### 比较并交换
此类操作的前缀为 CompareAndSwap，该操作简称 CAS，可以用来实现乐观锁。
```go
func CompareAndSwapInt32(addr *int32, old, new int32) (swapped bool)

func CompareAndSwapInt64(addr *int64, old, new int64) (swapped bool)

func CompareAndSwapPointer(addr *unsafe.Pointer, old, new unsafe.Pointer) (swapped bool)

func CompareAndSwapUint32(addr *uint32, old, new uint32) (swapped bool)

func CompareAndSwapUint64(addr *uint64, old, new uint64) (swapped bool)

func CompareAndSwapUintptr(addr *uintptr, old, new uintptr) (swapped bool)
```
该操作在进行交换前首先确保变量的值未被更改，即仍然保持参数 old 所记录的值，满足此前提下才进行交换操作。CAS 的做法类似操作数据库时常见的乐观锁机制。

需要注意的是，当有大量的 goroutine 对变量进行读写操作时，可能导致 CAS 操作无法成功，这时可以利用 for 循环多次尝试。

使用示例：
```go
func compareAndSwap(addr *int64, oldValue int64, newValue int64) {
    if atomic.CompareAndSwapInt64(addr, oldValue, newValue) {
        fmt.Println("cas opts: ", *addr)
        return
    }
}
```

#### 交换
此类操作的前缀为 Swap：
```go
func SwapInt32(addr *int32, new int32) (old int32)

func SwapInt64(addr *int64, new int64) (old int64)

func SwapPointer(addr *unsafe.Pointer, new unsafe.Pointer) (old unsafe.Pointer)

func SwapUint32(addr *uint32, new uint32) (old uint32)

func SwapUint64(addr *uint64, new uint64) (old uint64)

func SwapUintptr(addr *uintptr, new uintptr) (old uintptr)
```
相对于 CAS，明显此类操作更为暴力直接，并不管变量的旧值是否被改变，直接赋予新值然后返回背替换的值。

使用示例：
```go
func swap(addr *int64, newValue int64) {
    atomic.SwapInt64(addr, newValue)
    fmt.Println("swap opts: ", *addr)
}
```

#### 存储
此类操作的前缀为 Store：
```go
func StoreInt32(addr *int32, val int32)

func StoreInt64(addr *int64, val int64)

func StorePointer(addr *unsafe.Pointer, val unsafe.Pointer)

func StoreUint32(addr *uint32, val uint32)

func StoreUint64(addr *uint64, val uint64)

func StoreUintptr(addr *uintptr, val uintptr)

// 特殊类型： Value类型，常用于配置变更
func (v *Value) Store(x interface{})
```
此类操作确保了写变量的原子性，避免其他操作读到了修改变量过程中的脏数据。

使用示例：
```go
func store(addr *int64, newValue int64) {
    atomic.StoreInt64(addr, newValue)
    fmt.Println("store opts: ", *addr)
}
```

### 原子操作和锁的区别
- 原子操作由底层硬件支持，而锁是基于原子操作+信号量完成的。若实现相同的功能，前者通常会更有效率。
- 原子操作是单个指令的互斥操作；互斥锁/读写锁是一种数据结构，可以完成临界区（多个指令）的互斥操作，扩大原子操作的范围。
- 原子操作是无锁操作，属于乐观锁；说起锁的时候，一般属于悲观锁。
- 原子操作存在于各个指令/语言层级，比如“机器指令层级的原子操作”，“汇编指令层级的原子操作”，“Go语言层级的原子操作”等。
- 锁也存在于各个指令/语言层级中，比如“机器指令层级的锁”，“汇编指令层级的锁”，“Go语言层级的锁”等。
