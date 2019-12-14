/*
-------------------------------------------------
   Author :       zlyuan
   date：         2019/12/13
   Description :
-------------------------------------------------
*/

package zlru

import (
    "fmt"
    "strconv"
    "sync/atomic"
    "testing"
    "time"
)

// 测试获取和删除
func TestGetAndRemove(t *testing.T) {
    var tests = []struct {
        k string
        v interface{}
    }{
        {"k1", "a"},
        {"k2", "b"},
        {"k3", 3},
        {"k4", 4},
    }

    for _, o := range tests {
        lru := New(0, 0)
        lru.Add(o.k, o.v)
        if v, ok := lru.Get(o.k); !ok {
            t.Fatalf("获取失败")
        } else if v != o.v {
            t.Fatalf("收到了 %v, 它应该是 %s", v, o.v)
        }

        lru.Remove(o.k)
        if _, ok := lru.Get(o.k); ok {
            t.Fatal("返回了一个被删除的条目")
        }
    }
}

// 测试驱逐
func TestEvict(t *testing.T) {
    lru := New(0, 10)
    for i := 0; i < 13; i++ {
        lru.Add(fmt.Sprintf("tk%d", i), i)
    }

    if lru.Len() != 10 {
        t.Fatalf("得到了 %d 个条目; 它应该是 10", lru.Len())
    }
}

// 测试删除过旧的条目
func TestRemoveOldest(t *testing.T) {
    lru := New(0, 0)
    for i := 0; i < 13; i++ {
        lru.Add(fmt.Sprintf("tk%d", i), i)
    }

    lru.RemoveOldest(0, 3)

    if lru.Len() != 10 {
        t.Fatalf("得到了 %d 个条目; 它应该是 10", lru.Len())
    }
}

// 测试删除一段时间内未被使用的条目
func TestRemoveOldestOfTime(t *testing.T) {
    lru := New(0, 0)
    for i := 0; i < 13; i++ {
        lru.Add(fmt.Sprintf("tk%d", i), i)
    }

    time.Sleep(time.Millisecond * 200)
    for i := 3; i < 13; i++ {
        lru.Add(fmt.Sprintf("tk%d", i), i)
    }

    if count := lru.RemoveOldest(int64(time.Millisecond*100), 0); count != 3 {
        t.Fatalf("应该删除 3 个条目, 实际删除了 %d 个", count)
    }

    if lru.Len() != 10 {
        t.Fatalf("得到了 %d 个条目; 它应该是 10", lru.Len())
    }

    for i := 0; i < 3; i++ {
        k := fmt.Sprintf("tk%d", i)
        if v, ok := lru.Get(k); ok {
            t.Fatalf("%s:%s 应该是被删除的", k, v)
        }
    }
}

// 测试清空
func TestClear(t *testing.T) {
    lru := New(0, 0)
    for i := 0; i <= 10000; i++ {
        lru.Add(strconv.Itoa(i), i)
    }
    lru.Clear()
    if lru.Len() != 0 {
        t.Fatalf("得到了 %d 个条目; 它应该是 0", lru.Len())
    }
}

// 测试读取空条目
func Benchmark_ReadNil(b *testing.B) {
    lru := New(0, 0)
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            lru.Get(strconv.Itoa(i))
            i++
        }
    })
}

// 测试添加新条目
func Benchmark_AddNew(b *testing.B) {
    lru := New(0, 0)
    b.ResetTimer()
    n := int32(0)
    b.RunParallel(func(pb *testing.PB) {
        x := atomic.AddInt32(&n, 1)
        i := 0
        for pb.Next() {
            lru.Add(fmt.Sprintf("%d %d", x, i), i)
            i++
        }
    })
}

// 测试100%读
func Benchmark_Read100(b *testing.B) {
    benchmark_ReadPercent(b, 1)
}

// 测试100%写
func Benchmark_Add100(b *testing.B) {
    benchmark_ReadPercent(b, 0)
}

// 测试80%读, 20%写
func Benchmark_Read80(b *testing.B) {
    benchmark_ReadPercent(b, 0.8)
}

// 测试50%读, 50%写
func Benchmark_Read50(b *testing.B) {
    benchmark_ReadPercent(b, 0.5)
}

// 测试20%读, 80%写
func Benchmark_Read20(b *testing.B) {
    benchmark_ReadPercent(b, 0.2)
}

func benchmark_ReadPercent(b *testing.B, percent float32) {
    max := 100000
    line := int(float32(max) * percent)
    lru := New(0, 0)
    for i := 0; i <= max; i++ {
        lru.Add(strconv.Itoa(i), i)
    }

    b.ResetTimer()

    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            m := i % 100000
            if m < line {
                lru.Get(strconv.Itoa(m))
            } else {
                lru.Add(strconv.Itoa(m), i)
            }
            i++
        }
    })
}
