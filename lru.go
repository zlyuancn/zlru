/*
-------------------------------------------------
   Author :       zlyuan
   date：         2019/12/13
   Description :
-------------------------------------------------
*/

package zlru

import (
    "container/list"
    "crypto/rand"
    "hash/crc32"
    "math/big"
    "runtime"
    "sync"
    "sync/atomic"
    "time"
)

// 条目
type entry struct {
    last_time int64
    key       string
    value     interface{}
}

// LRU缓存, 它的所有方法都是非并发安全的, 需要调用者主动控制
type LruCache struct {
    count int64 // 当前条目数
    max   int64 // 最大条目数

    shard  uint32
    mxs    []*sync.Mutex
    lls    []*list.List               // 缓存链表
    caches []map[string]*list.Element // 条目key索引
}

// 新建一个缓存
// shard表示分片数量, 它将key做hash取模分配到指定的分片上
// max_entries表示最大条目数, 如果max_entries是0, 新增条目超出最大条目数时不会自动删除过旧的条目
func New(shard uint32, max_entries int64) *LruCache {
    if shard == 0 {
        shard = uint32(runtime.NumCPU())
    }

    mxs := make([]*sync.Mutex, shard)
    lls := make([]*list.List, shard)
    caches := make([]map[string]*list.Element, shard)

    for i := uint32(0); i < shard; i++ {
        mxs[i] = new(sync.Mutex)
        lls[i] = list.New()
        caches[i] = make(map[string]*list.Element)
    }

    return &lruCache{
        max:    max_entries,
        shard:  shard,
        mxs:    mxs,
        lls:    lls,
        caches: caches,
    }
}

func (m *LruCache) getMM(key string) (*sync.Mutex, *list.List, map[string]*list.Element) {
    hash := crc32.ChecksumIEEE([]byte(key))
    i := hash % m.shard
    return m.mxs[i], m.lls[i], m.caches[i]
}

// 添加一个条目
func (m *LruCache) Add(key string, value interface{}) {
    mx, ll, cache := m.getMM(key)
    mx.Lock()

    // 换位置
    if el, ok := cache[key]; ok {
        ll.MoveToFront(el)
        kv := el.Value.(*entry)
        kv.value = value
        kv.last_time = time.Now().UnixNano()
        mx.Unlock()
        return
    }

    // 新的key
    el := ll.PushFront(&entry{key: key, value: value, last_time: time.Now().UnixNano()})
    cache[key] = el
    count := atomic.AddInt64(&m.count, 1)
    mx.Unlock()

    // 超出最大数量则从尾部删除一个key
    if m.max > 0 && count > m.max {
        m.RemoveOldest(0, 1)
    }
}

// 获取指定key的条目的值
func (m *LruCache) Get(key string) (value interface{}, ok bool) {
    mx, ll, cache := m.getMM(key)
    mx.Lock()

    if el, got := cache[key]; got {
        ll.MoveToFront(el)
        kv := el.Value.(*entry)
        kv.last_time = time.Now().UnixNano()
        value, ok = kv.value, true
    }
    mx.Unlock()
    return
}

// 返回缓存中的条目数
func (m *LruCache) Len() int64 {
    return atomic.LoadInt64(&m.count)
}

// 删除指定key的条目
func (m *LruCache) Remove(key string) {
    mx, ll, cache := m.getMM(key)
    mx.Lock()
    if el, ok := cache[key]; ok {
        ll.Remove(el)
        delete(cache, key)
        atomic.AddInt64(&m.count, -1)
    }
    mx.Unlock()
}

// 删除一段时间内未被使用的条目, 最多删除max_count条
func (m *LruCache) RemoveOldest(t int64, max_count int) int {
    lifeline := time.Now().UnixNano() - t
    if max_count <= 0 {
        if t <= 0 {
            return m.Clear()
        }

        // 遍历删除所有t时间内未被使用的条目
        out := 0
        for s := uint32(0); s < m.shard; s++ {
            mx, ll, cache := m.mxs[s], m.lls[s], m.caches[s]
            mx.Lock()

            c := 0
            for {
                if el := ll.Back(); el != nil {
                    kv := el.Value.(*entry)
                    if kv.last_time < lifeline {
                        ll.Remove(el)
                        delete(cache, kv.key)
                        c++
                        out++
                        continue
                    }
                }
                break
            }

            if c > 0 {
                atomic.AddInt64(&m.count, int64(-c))
            }
            mx.Unlock()
        }
        return out
    }

    if int64(max_count) >= atomic.LoadInt64(&m.count) {
        return m.Clear()
    }

    loop := uint32(0)
    max_loop := uint32(float32(max_count) * 1.1)
    if max_loop < m.shard {
        max_loop = m.shard
    }

    out := 0
    sr := new(big.Int).SetInt64(int64(m.shard))
    for loop < max_loop && out < max_count {
        loop++
        n, _ := rand.Int(rand.Reader, sr)
        s := n.Int64()
        mx, ll, cache := m.mxs[s], m.lls[s], m.caches[s]

        mx.Lock()
        if el := ll.Back(); el != nil {
            kv := el.Value.(*entry)
            if t <= 0 || kv.last_time < lifeline {
                ll.Remove(el)
                delete(cache, kv.key)
                out++
                atomic.AddInt64(&m.count, -1)
            }
        }
        mx.Unlock()
    }
    return out
}

// 清除所有缓存, 注意: 并发下清空同时Add是允许的, 所以清空后的长度不一定是0
func (m *LruCache) Clear() int {
    out := 0
    for s := uint32(0); s < m.shard; s++ {
        mx, ll := m.mxs[s], m.lls[s]
        mx.Lock()
        c := ll.Len()
        out += c
        atomic.AddInt64(&m.count, int64(-c))

        ll = list.New()
        m.caches[s] = make(map[string]*list.Element)
        mx.Unlock()
    }
    return out
}

// 返回允许最大条目数
func (m *LruCache) MaxEntries() int64 {
    return m.max
}
