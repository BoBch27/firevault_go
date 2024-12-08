# Firevault Benchmarks

Firevault's validation performance was benchmarked against the industry standard Go validation package ([go-playground/validator](https://github.com/go-playground/validator)). 

Benchmarks were run on a machine with the following specifications:
- **Go**: go version go1.23.3 linux/amd64
- **CPU**: Intel(R) Core(TM) i7-1065G7 CPU @ 1.30GHz
- **OS**: Linux (kernel version)
- **Arch**: Amd64 

To reproduce these benchmarks, run `go test -bench=.`.

Below are the results:

Validation Benchmarks (with cache)
------------

| Test Case                              | Library                 | Operations | Time p/op (ns/op) |
|----------------------------------------|-------------------------|------------|-------------------|
| BenchmarkValidateSimpleStruct          | firevault               | 701,134    | 1,586             |
| BenchmarkValidateSimpleStruct          | go-playground/validator | 1,349,391  | 906.2             |
| BenchmarkValidateWithCustomRules       | firevault               | 637,363    | 1,659             |
| BenchmarkValidateWithCustomRules       | go-playground/validator | N/A        | N/A               |
| BenchmarkValidateStructWithSlice       | firevault               | 642,999    | 1,766             |
| BenchmarkValidateStructWithSlice       | go-playground/validator | 1,000,000  | 1,060             |
| BenchmarkValidateNestedStructWithSlice | firevault               | 427,381    | 2,552             |
| BenchmarkValidateNestedStructWithSlice | go-playground/validator | 1,086,962  | 1,206             |
| BenchmarkValidateSliceOfSimpleStructs  | firevault               | 242,559    | 4,933             |
| BenchmarkValidateSliceOfSimpleStructs  | go-playground/validator | 386,390    | 3,297             |

*The difference in performance, can be attributed to the fact Firevault extracts the validated data and creates a `map` from its values, which then gets passed on to Firestore, as well as the fact that it supports transformations. Without this functionality, results are much closer.*

Validation Benchmarks (without cache)
------------

| Test Case                              | Library                 | Operations | Time p/op (ns/op) |
|----------------------------------------|-------------------------|------------|-------------------|
| BenchmarkValidateSimpleStruct          | firevault               | 270,284    | 4,564             |
| BenchmarkValidateSimpleStruct          | go-playground/validator | 264,194    | 4,708             |
| BenchmarkValidateWithCustomRules       | firevault               | 220,111    | 5,494             |
| BenchmarkValidateWithCustomRules       | go-playground/validator | N/A        | N/A               |
| BenchmarkValidateStructWithSlice       | firevault               | 209,734    | 5,631             |
| BenchmarkValidateStructWithSlice       | go-playground/validator | 218,505    | 5,401             |
| BenchmarkValidateNestedStructWithSlice | firevault               | 114,862    | 8,493             |
| BenchmarkValidateNestedStructWithSlice | go-playground/validator | 122,714    | 8,987             |
| BenchmarkValidateSliceOfSimpleStructs  | firevault               | 88,987     | 14,744            |
| BenchmarkValidateSliceOfSimpleStructs  | go-playground/validator | 91,512     | 14,358            |
