# 基础

## 语言结构
---
![语言结构](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206042319229.png)

## 关键字
---
![关键字](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206042320183.png)

## 数据类型
---
![数据类型](https://gaofubao-image.oss-cn-shanghai.aliyuncs.com/202206042320681.png)

## 方法与函数区别
---
在 Go 语言中，函数和方法不太一样，有明确的概念区分。其他语言中，比如 Java，一般来说函数就是方法，方法就是函数；但是在 Go 语言中，函数是指不属于任何结构体、类型的方法，也就是说函数是没有接收者的；而方法是有接收者的。

方法：
```go
// 其中T是自定义类型或者结构体，不能是基础数据类型int等
func (t *T) add(a, b int) int {
	return a + b 
}
```

函数：
```go
func add(a, b int) int {
	return a + b 
}
```

## 方法值接收者与指针接收者区别
---
- 如果方法的接收者是指针类型，无论调用者是对象还是对象指针，修改的都是对象本身，会影响调用者；
- 如果方法的接收者是值类型，无论调用者是对象还是对象指针，修改的都是对象的副本，不影响调用者；

```go
type Person struct {
	age int
}

// 如果实现了接收者是指针类型的方法，会隐含地也实现了接收者是值类型的IncrAge1方法。
// 会修改age的值
func (p *Person) IncrAge1() {
	p.age += 1
}

// 如果实现了接收者是值类型的方法，会隐含地也实现了接收者是指针类型的IncrAge2方法。
// 不会修改age的值
func (p Person) IncrAge2() {
	p.age += 1
}

// 如果实现了接收者是值类型的方法，会隐含地也实现了接收者是指针类型的GetAge方法。
func (p Person) GetAge() int {
	return p.age
}

func main() {
	// p1 是值类型
	p := Person{age: 10}

	// 值类型 调用接收者是指针类型的方法
	p.IncrAge1()
	fmt.Println(p.GetAge())
	// 值类型 调用接收者是值类型的方法
	p.IncrAge2()
	fmt.Println(p.GetAge())

	// ----------------------

	// p2 是指针类型
	p2 := &Person{age: 20}

	// 指针类型 调用接收者是指针类型的方法
	p2.IncrAge1()
	fmt.Println(p2.GetAge())
	// 指针类型 调用接收者是值类型的方法
	p2.IncrAge2()
	fmt.Println(p2.GetAge())
}
```
上述代码中：
- 实现了接收者是指针类型的 IncrAge1 函数，不管调用者是值类型还是指针类型，都可以调用 IncrAge1 方法，并且它的 age 值都改变了。
- 实现了接收者是指针类型的 IncrAge2 函数，不管调用者是值类型还是指针类型，都可以调用 IncrAge2 方法，并且它的 age 值都没有被改变。

通常我们使用指针类型作为方法的接收者的理由：
- 使用指针类型能够修改调用者的值。
- 使用指针类型可以避免在每次调用方法时复制该值，在值的类型为大型结构体时，这样做会更加高效。

## 函数返回值
---
Go 函数返回局部变量的指针是否安全?

## 函数参数
---
Go 语言中所有的传参都是值传递（传值），都是一个副本，一个拷贝。
参数如果是非引用类型（int、string、struct等这些），这样就在函数中就无法修改原内容数据；如果是引用类型（指针、map、slice、chan等这些），这样就可以修改原内容数据。
是否可以修改原内容数据，和传值、传引用没有必然的关系。在 C++ 中，传引用肯定是可以修改原内容数据的，在 Go 语言里，虽然只有传值，但是我们也可以修改原内容数据，因为参数是引用类型。

- 什么是值传递？将实参的值传递给形参，形参是实参的一份拷贝，实参和形参的内存地址不同。函数内对形参值内容的修改，是否会影响实参的值内容，取决于参数是否是引用类型
- 什么是引用传递？将实参的地址传递给形参，函数内对形参值内容的修改，将会影响实参的值内容。Go 语言是没有引用传递的，在 C++ 中，函数参数的传递方式有引用传递。

> 引用类型和引用传递是2个概念，切记！！！

下面分别针对 Go 的值类型（int、struct等）、引用类型（指针、slice、map、channel），验证是否是值传递，以及函数内对形参的修改是否会修改原内容数据：

#### int 类型
形参和实际参数内存地址不一样，证明是指传递；参数是值类型，所以函数内对形参的修改，不会修改原内容数据。
```go
func main() {
	var i int64 = 1
	fmt.Printf("原始int内存地址是 %p\n", &i)

	modifyInt(i) // args就是实际参数
	fmt.Printf("改动后的值是: %v\n", i)
}

func modifyInt(i int64) { //这里定义的args就是形式参数
	fmt.Printf("函数里接收到int的内存地址是：%p\n", &i)
	i = 10
}
```
查看结果：
```shell
始int内存地址是 0xc0000b2008
函数里接收到int的内存地址是：0xc0000b2010
改动后的值是: 1
```

形参和实际参数内存地址不一样，证明是指传递，由于形参和实参是指针，指向同一个变量。函数内对指针指向变量的修改，会修改原内容数据。
```go
func main() {
	var args int64 = 1                  // int类型变量
	p := &args                          // 指针类型变量
	fmt.Printf("原始指针的内存地址是 %p\n", &p)   // 存放指针类型变量
	fmt.Printf("原始指针指向变量的内存地址 %p\n", p) // 存放int变量

	// args就是实际参数
	modifyPointer(p)
	fmt.Printf("改动后的值是: %v\n", *p)
}

// 这里定义的args就是形式参数
func modifyPointer(p *int64) {
	fmt.Printf("函数里接收到指针的内存地址是 %p \n", &p)
	fmt.Printf("函数里接收到指针指向变量的内存地址 %p\n", p)
	*p = 10
}
```
查看结果：
```shell
原始指针的内存地址是 0xc000124018
原始指针指向变量的内存地址 0xc00012a008
函数里接收到指针的内存地址是 0xc000124028 
函数里接收到指针指向变量的内存地址 0xc00012a008
改动后的值是: 10
```

#### slice 类型
形参和实际参数内存地址一样，不代表是引用类型；下面进行详细说明 slice 还是值传递，传递的是指针。
```go
func main() {
	var s = []int64{1, 2, 3}
	// &操作符打印出的地址是无效的，是fmt函数作了特殊处理
	fmt.Printf("直接对原始切片取地址%v \n", &s)
	// 打印slice的内存地址是可以直接通过%p打印的,不用使用&取地址符转换
	fmt.Printf("原始切片的内存地址： %p \n", s)
	fmt.Printf("原始切片第一个元素的内存地址： %p \n", &s[0])

	modifySlice(s)
	fmt.Printf("改动后的值是: %v\n", s)
}

func modifySlice(s []int64) {
	// &操作符打印出的地址是无效的，是fmt函数作了特殊处理
	fmt.Printf("直接对函数里接收到切片取地址%v\n", &s)
	// 打印slice的内存地址是可以直接通过%p打印的,不用使用&取地址符转换
	fmt.Printf("函数里接收到切片的内存地址是 %p \n", s)
	fmt.Printf("函数里接收到切片第一个元素的内存地址： %p \n", &s[0])
	s[0] = 10
}
```
查看结果：
```shell
直接对原始切片取地址&[1 2 3] 
原始切片的内存地址： 0xc0000b6000 
原始切片第一个元素的内存地址： 0xc0000b6000 
直接对函数里接收到切片取地址&[1 2 3]
函数里接收到切片的内存地址是 0xc0000b6000 
函数里接收到切片第一个元素的内存地址： 0xc0000b6000 
改动后的值是: [10 2 3]
```
slice 是一个结构体，他的第一个元素是一个指针类型，这个指针指向的是底层数组的第一个元素。当参数是 slice 类型的时候，fmt.printf 通过 %p 打印的 slice 变量的地址其实就是内部存储数组元素的地址，所以打印出来形参和实参内存地址一样。

```go
type slice struct {
    array unsafe.Pointer // 指针
    len   int
    cap   int
}
```
因为 slice 作为参数时本质是传递的指针，上面证明了指针也是值传递，所以参数为 slice 也是值传递，指针指向的是同一个变量，函数内对形参的修改，会修改原内容数据。
单纯的从 slice 结构体看，我们可以通过 modify 修改存储元素的内容，但是永远修改不了 len 和 cap，因为他们只是一个拷贝，如果要修改，那就要传递 &slice 作为参数才可以。

#### map 类型
形参和实际参数内存地址不一样，证明是值传递。
```go
func main() {
	m := make(map[string]int)
	m["age"] = 8
	fmt.Printf("原始map的内存地址是：%p\n", &m)

	modifyMap(m)
	fmt.Printf("改动后的值是: %v\n", m)
}

func modifyMap(m map[string]int) {
	fmt.Printf("函数里接收到map的内存地址是：%p\n", &m)
	m["age"] = 9
}
```
查看结果：
```shell
始map的内存地址是：0xc00000e028
函数里接收到map的内存地址是：0xc00000e038
改动后的值是: map[age:9]
```
通过 make 函数创建的 map 变量本质是一个 hmap 类型的指针 *hmap，所以函数内对形参的修改，会修改原内容数据。
```go
//src/runtime/map.go
func makemap(t *maptype, hint int, h *hmap) *hmap {
    mem, overflow := math.MulUintptr(uintptr(hint), t.bucket.size)
    if overflow || mem > maxAlloc {
        hint = 0
    }

    // initialize Hmap
    if h == nil {
        h = new(hmap)
    }
    h.hash0 = fastrand()
}
```

#### channel 类型
形参和实际参数内存地址不一样，证明是值传递。
```go
func main() {
	p := make(chan bool)
	fmt.Printf("原始chan的内存地址是：%p\n", &p)

	go func(p chan bool) {
		fmt.Printf("函数里接收到chan的内存地址是：%p\n", &p)
		//模拟耗时
		time.Sleep(2 * time.Second)
		p <- true
	}(p)

	select {
	case l := <-p:
		fmt.Printf("接收到的值是: %v\n", l)
	}
}
```
查看结果：
```shell
原始chan的内存地址是：0xc00000e028
函数里接收到chan的内存地址是：0xc00000e038
接收到的值是: true
```
通过 make 函数创建的 chan 变量本质是一个 hchan 类型的指针 *hchan，所以函数内对形参的修改，会修改原内容数据。
```go
// src/runtime/chan.go
func makechan(t *chantype, size int) *hchan {
    elem := t.elem

    // compiler checks this but be safe.
    if elem.size >= 1<<16 {
        throw("makechan: invalid channel element type")
    }
    if hchanSize%maxAlign != 0 || elem.align > maxAlign {
        throw("makechan: bad alignment")
    }

    mem, overflow := math.MulUintptr(elem.size, uintptr(size))
    if overflow || mem > maxAlloc-hchanSize || size < 0 {
        panic(plainError("makechan: size out of range"))
    }
}
```

#### struct类型
形参和实际参数内存地址不一样，证明是值传递。形参不是引用类型或者指针类型，所以函数内对形参的修改，不会修改原内容数据。
```go
type Person struct {
	Name string
	Age  int
}

func main() {
	per := Person{
		Name: "test",
		Age:  8,
	}
	fmt.Printf("原始struct的内存地址是：%p\n", &per)
	modifyStruct(per)
	fmt.Printf("改动后的值是: %v\n", per)
}

func modifyStruct(per Person) {
	fmt.Printf("函数里接收到struct的内存地址是：%p\n", &per)
	per.Age = 10
}
```
查看结果：
```shell
原始struct的内存地址是：0xc0000a4018
函数里接收到struct的内存地址是：0xc0000a4030
改动后的值是: {test 8}
```


## defer 实现原理
---
定义：
defer 能够让我们推迟执行某些函数调用，推迟到当前函数返回前才实际执行。defer 与 panic 和 recover 结合，形成了 Go 语言风格的异常与捕获机制。

使用场景：
defer 语句经常被用于处理成对的操作，如文件句柄关闭、连接关闭、释放锁

优点：方便开发者使用
缺点：有性能损耗

实现原理：
Go1.14 中编译器会将 defer 函数直接插入到函数的尾部，无需链表和栈上参数拷贝，性能大幅提升。把 defer 函数在当前函数内展开并直接调用，这种方式被称为 open coded defer。

```go
// 源代码
func A(i int) {
    defer A1(i, 2*i)
    if(i > 1) {
        defer A2("Hello", "eggo")
    }
    // code to do something
    return
}

func A1(a,b int) {
    //...
}

func A2(m,n string) {
    //...
}

// 编译后（伪代码）
func A(i int) {
    // code to do something
    if(i > 1){
       A2("Hello", "eggo")
    }
    A1(i, 2*i)
    return
}
```

1. 函数退出前，按照先进后出的顺序，执行 defer 函数
```go
// defer：延迟函数执行，先进后出
func main() {
	defer fmt.Println("defer1")
	defer fmt.Println("defer2")
	defer fmt.Println("defer3")
	defer fmt.Println("defer4")
	fmt.Println("end")
}
```
查看结果：
```shell
end
defer4
defer3
defer2
defer1
```

2. panic 后的 defer 函数不会被执行（遇到 panic，如果没有捕获错误，函数会立刻终止）
```go
// panic后的defer函数不会被执行
func main() {
	defer fmt.Println("panic before")
	panic("发生panic")

	defer func() {
		fmt.Println("panic after")
	}()
}
```
查看结果：
```shell
panic before
panic: 发生panic
```

3. panic 没有被 recover 时，抛出的 panic 到当前 goroutine 最上层函数时，最上层程序直接异常终止
```go
// 子函数抛出的panic没有recover时，上层函数时，程序直接异常终止
func main() {
	defer func() {
		fmt.Println("c")
	}()
	F()
	fmt.Println("继续执行")
}

func F() {
	defer func() {
		fmt.Println("b")
	}()
	panic("a")
}
```
查看结果：
```shell
b
c
panic: a
```

4. panic 有被 recover 时，当前 goroutine 最上层函数正常执行
```go
func main() {
	defer func() {
		fmt.Println("c")
	}()
	F()
	fmt.Println("继续执行")
}

func F() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("捕获异常:", err)
		}
		fmt.Println("b")
	}()
	panic("a")
}
```
查看结果：
```shell
捕获异常: a
b
继续执行
c
```

## make 与 new 区别
---
首先纠正下 make 和 new 是内置函数，不是关键字。
变量初始化，一般包括2步，变量声明 + 变量内存分配，var 关键字就是用来声明变量的，new 和 make 函数主要是用来分配内存的。

var 声明值类型的变量时，系统会默认为他分配内存空间，并赋该类型的零值，比如布尔、数字、字符串、结构体。如果指针类型或者引用类型的变量，系统不会为它分配内存，默认就是nil。此时如果你想直接使用，那么系统会抛异常，必须进行内存分配后，才能使用。

new 和 make 两个内置函数，主要用来分配内存空间，有了内存，变量就能使用了，主要有以下2点区别：
- 使用场景区别：
	- make 只能用来分配及初始化类型为 slice、map、chan 的数据。
	- new 可以分配任意类型的数据，并且置零。
- 返回值区别：
	- make 函数原型如下，返回的是 slice、map、chan 类型本身，这 3 种类型是引用类型，就没有必要返回他们的指针。`func make(t Type, size ...IntegerType) Type`
	- new 函数原型如下，返回一个指向该类型内存地址的指针。`func new(Type) *Type`
