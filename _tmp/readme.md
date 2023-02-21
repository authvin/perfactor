Bench the original solution:
 > go test -mode 0 -bench=Either -run=NONE -benchmem -memprofile mem.pprof -cpuprofile cpu.pprof > origin.bench
Bench the target solution:
 > go test -mode 1 -bench=Either -run=NONE -benchmem -memprofile mem.pprof -cpuprofile cpu.pprof > target.bench
Compare the benchmarks:
 > benchcmp origin.bench target.bench