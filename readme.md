# 高性能的LRU缓存库, 并发安全

---

# 获得
`go get -u github.com/zlyuancn/zlru`

# 以下是性能测试数据
```
2.50GHz * 16
Benchmark_ReadNil-16    	13902390	       76.5 ns/op
Benchmark_AddNew-16     	 1833960	       691 ns/op
Benchmark_Read100-16    	 7647756	       171 ns/op
Benchmark_Add100-16     	 6982910	       186 ns/op
Benchmark_Read80-16     	 7340974	       169 ns/op
Benchmark_Read50-16     	 7969102	       173 ns/op
Benchmark_Read20-16     	 8084581	       183 ns/op
```