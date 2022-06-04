# Map

## map 实现原理
---
Go 中的 map 是一个指针，占用 8 个字节，指向 hmap 结构体。
源码包中 src/runtime/map.go 定义了 hmap 的数据结构，hmap 包含若干个结构为 bmap 的数组，每个 bmap 底层都采用链表结构，bmap 通常叫其 bucket。

![map](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206042135569.png)

### hmap 结构体
```go
// A header for a Go map.
type hmap struct {
    count     int 
    // 代表哈希表中的元素个数，调用len(map)时，返回的就是该字段值。
    flags     uint8 
    // 状态标志（是否处于正在写入的状态等）
    B         uint8  
    // buckets（桶）的对数
    // 如果B=5，则buckets数组的长度 = 2^B=32，意味着有32个桶
    noverflow uint16 
    // 溢出桶的数量
    hash0     uint32 
    // 生成hash的随机数种子
    buckets    unsafe.Pointer 
    // 指向buckets数组的指针，数组大小为2^B，如果元素个数为0，它为nil。
    oldbuckets unsafe.Pointer 
    // 如果发生扩容，oldbuckets是指向老的buckets数组的指针，老的buckets数组大小是新的buckets的1/2;非扩容状态下，它为nil。
    nevacuate  uintptr        
    // 表示扩容进度，小于此地址的buckets代表已搬迁完成。
    extra *mapextra 
    // 存储溢出桶，这个字段是为了优化GC扫描而设计的，下面详细介绍
}
```

### bmap 结构体
bmap 就是我们常说的“桶”，一个桶里面会最多装 8 个 key，这些 key 之所以会落入同一个桶，是因为它们经过哈希计算后，哈希结果的低 8 位是相同的，关于 key 的定位我们在 map 的查询中详细说明。在桶内，又会根据 key 计算出来的 hash 值的高 8 位来决定 key 到底落入桶内的哪个位置（一个桶内最多有 8 个位置)。
```go
// A bucket for a Go map.
type bmap struct {
    tophash [bucketCnt]uint8        
    // len为8的数组
    // 用来快速定位key是否在这个bmap中
    // 一个桶最多8个槽位，如果key所在的tophash值在tophash中，则代表该key在这个桶中
}
```
上面 bmap 结构是静态结构，在编译过程中 runtime.bmap 会拓展成以下结构体：
```go
type bmap struct{
    tophash [8]uint8
    keys [8]keytype 
    // keytype 由编译器编译时候确定
    values [8]elemtype 
    // elemtype 由编译器编译时候确定
    overflow uintptr 
    // overflow指向下一个bmap，overflow是uintptr而不是*bmap类型，保证bmap完全不含指针，是为了减少gc，溢出桶存储到extra字段中
}
```
tophash 就是用于实现快速定位 key 的位置，在实现过程中会使用 key 的 hash 值的高 8 位作为 tophash 值，存放在 bmap 的 tophash 字段中。
tophash 字段不仅存储 key 哈希值的高 8 位，还会存储一些状态值，用来表明当前桶单元状态，这些状态值都是小于 minTopHas h的。
为了避免 key 哈希值的高 8 位值和这些状态值相等，产生混淆情况，所以当 ke y哈希值高 8 位若小于 minTopHash 时候，自动将其值加上 minTopHash 作为该 key 的 tophash。桶单元的状态值如下：
```go
emptyRest      = 0 // 表明此桶单元为空，且更高索引的单元也是空
emptyOne       = 1 // 表明此桶单元为空
evacuatedX     = 2 // 用于表示扩容迁移到新桶前半段区间
evacuatedY     = 3 // 用于表示扩容迁移到新桶后半段区间
evacuatedEmpty = 4 // 用于表示此单元已迁移
minTopHash     = 5 // key的tophash值与桶状态值分割线值，小于此值的一定代表着桶单元的状态，大于此值的一定是key对应的tophash值

func tophash(hash uintptr) uint8 {
    top := uint8(hash >> (goarch.PtrSize*8 - 8))
    if top < minTopHash {
        top += minTopHash
    }
    return top
}
```
### mapextra 结构体
当 map 的 key 和 value 都不是指针类型时候，bmap 将完全不包含指针，那么 gc 时候就不用扫描 bmap。bmap 指向溢出桶的字段 overflow 是 uintptr 类型，为了防止这些 overflow 桶被 gc 掉，所以需要 mapextra.overflow 将它保存起来。如果 bmap 的 overflow 是 *bmap 类型，那么 gc 扫描的是一个个拉链表，效率明显不如直接扫描一段内存（hmap.mapextra.overflow）。
```go
type mapextra struct {
    overflow    *[]*bmap
    // overflow 包含的是 hmap.buckets 的 overflow 的 buckets
    oldoverflow *[]*bma
    // oldoverflow 包含扩容时 hmap.oldbuckets 的 overflow 的 bucket
    nextOverflow *bmap 
    // 指向空闲的 overflow bucket 的指针
}
```
### 总结
bmap（bucket）内存数据结构可视化如下：
![bmap](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206042143261.png)
注意到 key 和 value 是各自放在一起的，并不是 key/value/key/value/... 这样的形式，当 key 和 value 类型不一样的时候，key 和 value 占用字节大小不一样，使用 key/value 这种形式可能会因为内存对齐导致内存空间浪费，所以 Go 采用 key 和 value 分开存储的设计，更节省内存空间。

## map 遍历
---
使用 range 多次遍历 map 时输出的 key 和 value 的顺序可能不同。这是 Go 语言的设计者们有意为之，旨在提示开发者们，Go 底层实现并不保证 map 遍历顺序稳定，请大家不要依赖 range 遍历结果顺序。

主要原因有2点：
- map 在遍历时，并不是从固定的 0 号 bucket 开始遍历的，每次遍历，都会从一个随机值序号的 bucket，再从其中随机的 cell 开始遍历。
- map 遍历时，是按序遍历 bucket，同时按需遍历 bucket 中和其 overflow bucket 中的 cell。但是 map 在扩容后，会发生 key 的搬迁，这造成原来落在一个 bucket 中的 key，搬迁后，有可能会落到其他 bucket 中了，从这个角度看，遍历 map 的结果就不可能是按照原来的顺序了。

map 本身是无序的，且遍历时顺序还会被随机化，如果想顺序遍历 map，需要对 map key 先排序，再按照 key 的顺序遍历 map。

```go
func main() {
	m := map[int]string{1: "a", 2: "b", 3: "c"}
	fmt.Println("first range:")
	for i, v := range m {
		fmt.Printf("m[%v]=%v\n", i, v)
	}
	fmt.Println("second range:")
	for i, v := range m {
		fmt.Printf("m[%v]=%v\n", i, v)
	}

	// 实现有序遍历
	var sl []int
	// 把 key 单独取出放到切片
	for k := range m {
		sl = append(sl, k)
	}
	// 排序切片
	sort.Ints(sl)
	// 以切片中的 key 顺序遍历 map 就是有序的了
	for _, k := range sl {
		fmt.Println(k, m[k])
	}
}
```

## map 非线程安全
---
map 默认是并发不安全的，同时对 map 进行并发读写时，程序会 panic，原因如下：
Go 官方在经过了长时间的讨论后，认为 Go map 更应适配典型使用场景（不需要从多个 goroutine 中进行安全访问），而不是为了小部分情况（并发访问），导致大部分程序付出加锁代价（性能），决定了不支持。

场景：2 个协程同时读和写，以下程序会出现致命错误：fatal error: concurrent map writes
```go
func main() {
	s := make(map[int]int)

	for i := 0; i < 100; i++ {
		go func(i int) {
			s[i] = i
		}(i)
	}

	for i := 0; i < 100; i++ {
		go func(i int) {
			fmt.Printf("map第%d个元素值是%d\n", i, s[i])
		}(i)
	}
	time.Sleep(1 * time.Second)
}
```

如果想实现 map 线程安全，有两种方式：
方式一：使用读写锁
```go
func main() {
	var lock sync.RWMutex
	s := make(map[int]int)
	for i := 0; i < 100; i++ {
		go func(i int) {
			lock.Lock()
			s[i] = i
			lock.Unlock()
		}(i)
	}
	for i := 0; i < 100; i++ {
		go func(i int) {
			lock.RLock()
			fmt.Printf("map第%d个元素值是%d\n", i, s[i])
			lock.RUnlock()
		}(i)
	}
	time.Sleep(1 * time.Second)
}
```

方式二：使用 sync.Map
```go
func main() {
	var m sync.Map

	for i := 0; i < 100; i++ {
		go func(i int) {
			m.Store(i, i)
		}(i)
	}

	for i := 0; i < 100; i++ {
		go func(i int) {
			v, ok := m.Load(i)
			fmt.Printf("Load: %v, %v\n", v, ok)
		}(i)
	}
	time.Sleep(1 * time.Second)
}
```

## map 查找
---
Go 语言中读取 map 有两种语法：带 comma 和 不带 comma。当要查询的 key 不在 map 里，带 comma 的用法会返回一个 bool 型变量提示 key 是否在 map 中；而不带 comma 的语句则会返回一个 value 类型的零值。如果 value 是 int 型就会返回 0，如果 value 是 string 类型，就会返回空字符串。

```go
// 不带 comma 用法
value := m["name"]
fmt.Printf("value:%s", value)

// 带 comma 用法
value, ok := m["name"]
if ok {
    fmt.Printf("value:%s", value)
}
```

map 的查找通过生成汇编码可以知道，根据 key 的不同类型/返回参数，编译器会将查找函数用更具体的函数替换，以优化效率：

| key    | 类型                                             | 查找                   |
| ------ | ------------------------------------------------ | ---------------------- |
| uint32 | mapaccess1_fast32(t maptype, h hmap, key uint32) | unsafe.Pointer         |
| uint32 | mapaccess2_fast32(t maptype, h hmap, key uint32) | (unsafe.Pointer, bool) |
| uint64 | mapaccess1_fast64(t maptype, h hmap, key uint64) | unsafe.Pointer         |
| uint64 | mapaccess2_fast64(t maptype, h hmap, key uint64) | (unsafe.Pointer, bool) |
| string | mapaccess1_faststr(t maptype, h hmap, ky string) | unsafe.Pointer         |
| string | mapaccess2_faststr(t maptype, h hmap, ky string) | (unsafe.Pointer, bool) |

查找流程：
![查找流程](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206042203661.png)

1. 写保护监测
函数首先会检查 map 的标志位 flags。如果 flags 的写标志位此时被置 1 了，说明有其他协程在执行“写”操作，进而导致程序 panic，这也说明了 map 不是线程安全的。
```go
if h.flags&hashWriting != 0 {
    throw("concurrent map read and map write")
}
```
2. 计算 hash 值
```go
hash := t.hasher(key, uintptr(h.hash0))
```
key 经过哈希函数计算后，得到的哈希值如下（主流 64 位机下共 64 个 bit 位）， 不同类型的 key 会有不同的hash函数：
```go
10010111 | 000011110110110010001111001010100010010110010101010 │ 01010
```
3. 找到 hash 对应的 bucket
bucket 定位：哈希值的低 8 个 bit 位，用来定位 key 所存放的 bucket。
如果当前正在扩容中，并且定位到的旧 bucket 数据还未完成迁移，则使用旧的 bucket（扩容前的 bucket）。
```go
hash := t.hasher(key, uintptr(h.hash0))
// 桶的个数m-1，即 1<<B-1,B=5时，则有0~31号桶
m := bucketMask(h.B)
// 计算哈希值对应的bucket
// t.bucketsize为一个bmap的大小，通过对哈希值和桶个数取模得到桶编号，通过对桶编号和buckets起始地址进行运算，获取哈希值对应的bucket
b := (*bmap)(add(h.buckets, (hash&m)*uintptr(t.bucketsize)))
// 是否在扩容
if c := h.oldbuckets; c != nil {
    // 桶个数已经发生增长一倍，则旧bucket的桶个数为当前桶个数的一半
    if !h.sameSizeGrow() {
        // There used to be half as many buckets; mask down one more power of two.
        m >>= 1
    }
    // 计算哈希值对应的旧bucket
    oldb := (*bmap)(add(c, (hash&m)*uintptr(t.bucketsize)))
    // 如果旧bucket的数据没有完成迁移，则使用旧bucket查找
    if !evacuated(oldb) {
        b = oldb
    }
}
```
4. 遍历 bucket 查找
tophash 值定位：哈希值的高 8 个 bit 位，用来快速判断 key 是否已在当前 bucket 中（如果不在的话，需要去 bucket 的 overflow 中查找）。
用步骤 2 中的 hash 值，得到高 8 个 bit 位，也就是 10010111，转化为十进制，也就是 151。
```go
top := tophash(hash)
func tophash(hash uintptr) uint8 {
    top := uint8(hash >> (goarch.PtrSize*8 - 8))
    if top < minTopHash {
        top += minTopHash
    }
    return top
}
```
上面函数中 hash 是 64 位的，sys.PtrSize 值是 8，所以 top := uint8(hash >> (sys.PtrSize*8 - 8)) 等效 top = uint8(hash >> 56)，最后 top 取出来的值就是 hash 的高 8 位值。
在 bucket 及 bucket的overflow中寻找tophash 值（HOB hash）为 151* 的槽位，即为 key 所在位置，找到了空槽位或者 2 号槽位，这样整个查找过程就结束了，其中找到空槽位代表没找到。

![查找](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206042210898.png)

```go
for ; b != nil; b = b.overflow(t) {
    for i := uintptr(0); i < bucketCnt; i++ {
        if b.tophash[i] != top {
            // 未被使用的槽位，插入
            if b.tophash[i] == emptyRest {
                break bucketloop
            }
            continue
        }
        // 找到tophash值对应的的key
        k := add(unsafe.Pointer(b), dataOffset+i*uintptr(t.keysize))
        if t.key.equal(key, k) {
            e := add(unsafe.Pointer(b), dataOffset+bucketCnt*uintptr(t.keysize)+i*uintptr(t.elemsize))
            return e
        }
    }
}
```

5. 返回 key 对应的指针
如果通过上面的步骤找到了 key 对应的槽位下标 i，我们再详细分析下 key/value 值是如何获取的：
```go
// keys的偏移量
dataOffset = unsafe.Offsetof(struct{
  b bmap
  v int64
}{}.v)

// 一个bucket的元素个数
bucketCnt = 8

// key 定位公式
k :=add(unsafe.Pointer(b),dataOffset+i*uintptr(t.keysize))

// value 定位公式
v:= add(unsafe.Pointer(b),dataOffset+bucketCnt*uintptr(t.keysize)+i*uintptr(t.valuesize))
```
bucket 里 keys 的起始地址就是 unsafe.Pointer(b)+dataOffset。
第 i 个下标 key 的地址就要在此基础上跨过 i 个 key 的大小；
而我们又知道，value 的地址是在所有 key 之后，因此第 i 个下标 value 的地址还需要加上所有 key 的偏移。

## map 冲突
---
### 哈希冲突解决方案
比较常用的Hash冲突解决方案有链地址法和开放寻址法：
- 链地址法：当哈希冲突发生时，创建新单元，并将新单元添加到冲突单元所在链表的尾部。
- 开放寻址法：当哈希冲突发生时，从发生冲突的那个单元起，按照一定的次序，从哈希表中寻找一个空闲的单元，然后把发生冲突的元素存入到该单元。开放寻址法需要的表长度要大于等于所需要存放的元素数量。

开放寻址法有多种方式：线性探测法、平方探测法、随机探测法和双重哈希法。这里以线性探测法来帮助读者理解开放寻址法思想

线性探测法
设 Hash(key) 表示关键字 key 的哈希值， 表示哈希表的槽位数（哈希表的大小）。
线性探测法则可以表示为：
- 如果 Hash(x) % M 已经有数据，则尝试 (Hash(x) + 1) % M ;
- 如果 (Hash(x) + 1) % M 也有数据了，则尝试 (Hash(x) + 2) % M ;
- 如果 (Hash(x) + 2) % M 也有数据了，则尝试 (Hash(x) + 3) % M ;

两种解决方案比较
- 对于链地址法，基于数组 + 链表进行存储，链表节点可以在需要时再创建，不必像开放寻址法那样事先申请好足够内存，因此链地址法对于内存的利用率会比开方寻址法高。链地址法对装载因子的容忍度会更高，并且适合存储大对象、大数据量的哈希表。而且相较于开放寻址法，它更加灵活，支持更多的优化策略，比如可采用红黑树代替链表。但是链地址法需要额外的空间来存储指针。
- 对于开放寻址法，它只有数组一种数据结构就可完成存储，继承了数组的优点，对CPU缓存友好，易于序列化操作。但是它对内存的利用率不如链地址法，且发生冲突时代价更高。当数据量明确、装载因子小，适合采用开放寻址法。

### 总结
在发生哈希冲突时，Python 中 dict 采用的开放寻址法，Java 的 HashMap 采用的是链地址法，而 Go map 也采用链地址法解决冲突，具体就是插入 key 到 map 中时，当 key 定位的桶填满 8 个元素后（这里的单元就是桶，不是元素），将会创建一个溢出桶，并且将溢出桶插入当前桶所在链表尾部。
```go
if inserti == nil {
    // all current buckets are full, allocate a new one.
    newb := h.newoverflow(t, b)
    // 创建一个新的溢出桶
    inserti = &newb.tophash[0]
    insertk = add(unsafe.Pointer(newb), dataOffset)
    elem = add(insertk, bucketCnt*uintptr(t.keysize))
}
```

## map 负载因子
---
负载因子（load factor），用于衡量当前哈希表中空间占用率的核心指标，也就是每个 bucket 桶存储的平均元素个数。
> 负载因子 = 哈希表存储的元素个数/桶个数

另外负载因子与扩容、迁移等重新散列（rehash）行为有直接关系：
- 在程序运行时，会不断地进行插入、删除等，会导致 bucket 不均，内存利用率低，需要迁移。
- 在程序运行时，出现负载因子过大，需要做扩容，解决 bucket 过大的问题。

负载因子是哈希表中的一个重要指标，在各种版本的哈希表实现中都有类似的东西，主要目的是为了平衡 buckets 的存储空间大小和查找元素时的性能高低。
在接触各种哈希表时都可以关注一下，做不同的对比，看看各家的考量。

为什么 Go 语言中哈希表的负载因子是 6.5，为什么不是 8 ，也不是 1。这里面有可靠的数据支撑吗？
实际上这是 Go 官方的经过认真的测试得出的数字，一起来看看官方的这份测试报告。
报告中共包含 4 个关键指标，如下：

| loadFactor | %overflow | bytes/entry | hitprobe | missprobe |
| ---------- | --------- | ----------- | -------- | --------- |
| 4.00       | 2.13      | 20.77       | 3.00     | 4.00      |
| 4.50       | 4.05      | 17.30       | 3.25     | 4.50      |
| 5.00       | 6.85      | 14.77       | 3.50     | 5.00      |
| 5.50       | 10.55     | 12.94       | 3.75     | 5.50      |
| 6.00       | 15.27     | 11.67       | 4.00     | 6.00      |
| 6.50       | 20.90     | 10.79       | 4.25     | 6.50      |
| 7.00       | 27.14     | 10.15       | 4.50     | 7.00      |
| 7.50       | 34.03     | 9.73        | 4.75     | 7.50      |
| 8.00       | 41.10     | 9.40        | 5.00     | 8.00      |

- loadFactor：负载因子，也有叫装载因子。
- %overflow：溢出率，有溢出 bukcet 的百分比。
- bytes/entry：平均每对 key/value 的开销字节数。
- hitprobe：查找一个存在的 key 时，要查找的平均个数。
- missprobe：查找一个不存在的 key 时，要查找的平均个数。

Go 官方发现：装载因子越大，填入的元素越多，空间利用率就越高，但发生哈希冲突的几率就变大。反之，装载因子越小，填入的元素越少，冲突发生的几率减小，但空间浪费也会变得更多，而且还会提高扩容操作的次数
根据这份测试结果和讨论，Go 官方取了一个相对适中的值，把 Go 中的 map 的负载因子硬编码为 6.5，这就是 6.5 的选择缘由。
这意味着在 Go 语言中，当 map存储的元素个数大于或等于 6.5 * 桶个数时，就会触发扩容行为。

## map 扩容
---
### 扩容时机
在向 map 插入新 key 的时候，会进行条件检测，符合下面这 2 个条件，就会触发扩容：
```go
if !h.growing() && (overLoadFactor(h.count+1, h.B) || tooManyOverflowBuckets(h.noverflow, h.B)) {
    hashGrow(t, h)
    goto again // Growing the table invalidates everything, so try again
}

// 判断是否在扩容
func (h *hmap) growing() bool {
    return h.oldbuckets != nil
}
```
条件1：超过负载
map元素个数 > 6.5 * 桶个数
```go
func overLoadFactor(count int, B uint8) bool {
   return count > bucketCnt && uintptr(count) > loadFactor*bucketShift(B)
}
```
其中：
- bucketCnt = 8，一个桶可以装的最大元素个数
- loadFactor = 6.5，负载因子，平均每个桶的元素个数
- bucketShift(B): 桶的个数

条件2：溢出桶太多
当桶总数 < 2 ^ 15 时，如果溢出桶总数 >= 桶总数，则认为溢出桶过多。
当桶总数 >= 2 ^ 15 时，直接与 2 ^ 15 比较，当溢出桶总数 >= 2 ^ 15 时，即认为溢出桶太多了。
```go
func tooManyOverflowBuckets(noverflow uint16, B uint8) bool {
    // If the threshold is too low, we do extraneous work.
    // If the threshold is too high, maps that grow and shrink can hold on to lots of unused memory.
    // "too many" means (approximately) as many overflow buckets as regular buckets.
    // See incrnoverflow for more details.
    if B > 15 {
        B = 15
    }
    // The compiler doesn't see here that B < 16; mask B to generate shorter shift code.
    return noverflow >= uint16(1)<<(B&15)
}
```
对于条件 2，其实算是对条件 1 的补充。因为在负载因子比较小的情况下，有可能 map 的查找和插入效率也很低，而第 1 点识别不出来这种情况。
表面现象就是负载因子比较小比较小，即 map 里元素总数少，但是桶数量多（真实分配的桶数量多，包括大量的溢出桶）。比如不断的增删，这样会造成 overflow 的 bucket 数量增多，但负载因子又不高，达不到第 1 点的临界值，就不能触发扩容来缓解这种情况。这样会造成桶的使用率不高，值存储得比较稀疏，查找插入效率会变得非常低，因此有了第 2 扩容条件。

### 扩容机制
- 双倍扩容：针对条件 1，新建一个 buckets 数组，新的 buckets 大小是原来的 2 倍，然后旧 buckets 数据搬迁到新的 buckets。该方法我们称之为双倍扩容。
- 等量扩容：针对条件 2，并不扩大容量，buckets 数量维持不变，重新做一遍类似双倍扩容的搬迁动作，把松散的键值对重新排列一次，使得同一个 bucket 中的 key 排列地更紧密，节省空间，提高 bucket 利用率，进而保证更快的存取。该方法我们称之为等量扩容。

### 扩容函数
上面说的 hashGrow() 函数实际上并没有真正地“搬迁”，它只是分配好了新的 buckets，并将老的 buckets 挂到了 oldbuckets 字段上。真正搬迁 buckets 的动作在 growWork() 函数中，而调用 growWork() 函数的动作是在 mapassign 和 mapdelete 函数中。也就是插入或修改、删除 key 的时候，都会尝试进行搬迁 buckets 的工作。先检查 oldbuckets 是否搬迁完毕，具体来说就是检查 oldbuckets 是否为 nil。
```go
func hashGrow(t *maptype, h *hmap) {
    // 如果达到条件 1，那么将B值加1，相当于是原来的2倍
    // 否则对应条件 2，进行等量扩容，所以 B 不变
    bigger := uint8(1)
    if !overLoadFactor(h.count+1, h.B) {
        bigger = 0
        h.flags |= sameSizeGrow
    }
    // 记录老的buckets
    oldbuckets := h.buckets
    // 申请新的buckets空间
    newbuckets, nextOverflow := makeBucketArray(t, h.B+bigger, nil)
    // 注意&^ 运算符，这块代码的逻辑是转移标志位
    flags := h.flags &^ (iterator | oldIterator)
    if h.flags&iterator != 0 {
        flags |= oldIterator
    }
    // 提交grow (atomic wrt gc)
    h.B += bigger
    h.flags = flags
    h.oldbuckets = oldbuckets
    h.buckets = newbuckets
    // 搬迁进度为0
    h.nevacuate = 0
    // overflow buckets 数为0
    h.noverflow = 0

    // 如果发现hmap是通过extra字段 来存储 overflow buckets时
    if h.extra != nil && h.extra.overflow != nil {
        if h.extra.oldoverflow != nil {
            throw("oldoverflow is not nil")
        }
        h.extra.oldoverflow = h.extra.overflow
        h.extra.overflow = nil
    }
    if nextOverflow != nil {
        if h.extra == nil {
            h.extra = new(mapextra)
        }
        h.extra.nextOverflow = nextOverflow
    }
}
```
由于 map 扩容需要将原有的 key/value 重新搬迁到新的内存地址，如果 map 存储了数以亿计的 key-value，一次性搬迁将会造成比较大的延时，因此 Go map 的扩容采取了一种称为“渐进式”的方式，原有的 key 并不会一次性搬迁完毕，每次最多只会搬迁 2 个 bucket。
```go
func growWork(t *maptype, h *hmap, bucket uintptr) {
    // 为了确认搬迁的 bucket 是我们正在使用的 bucket
    // 即如果当前key映射到老的bucket1，那么就搬迁该bucket1。
    evacuate(t, h, bucket&h.oldbucketmask())
    // 如果还未完成扩容工作，则再搬迁一个bucket。
    if h.growing() {
        evacuate(t, h, h.nevacuate)
    }
}
```

## map 与 sync.Map 性能对比
---
Go 语言的 sync.Map 支持并发读写，采取了“空间换时间”的机制，冗余了两个数据结构，分别是：read 和 dirty。
```go
type Map struct {
    mu      Mutex
    read    atomic.Value // readOnly
    dirty   map[interface{}]*entry
    misses  int
}
```
对比原始 map：
和原始 map+RWLock 的实现并发的方式相比，减少了加锁对性能的影响。它做了一些优化：可以无锁访问 read map，而且会优先操作 read map，倘若只操作 read map 就可以满足要求，那就不用去操作 write map(dirty)，所以在某些特定场景中它发生锁竞争的频率会远远小于 map+RWLock 的实现方式

- 优点：适合读多写少的场景
- 缺点：写多的场景，会导致 read map 缓存失效，需要加锁，冲突变多，性能急剧下降