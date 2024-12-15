# Firevault Benchmarks

Firevault's validation performance was benchmarked against the industry standard Go validation package - [go-playground/validator](https://github.com/go-playground/validator) (v10.23.0).

Benchmarks were run on a machine with the following specifications:
- **Go**: go version go1.23.3 linux/amd64
- **CPU**: Intel(R) Core(TM) i7-1065G7 CPU @ 1.30GHz
- **OS**: Linux (kernel version)
- **Arch**: Amd64 

To reproduce these benchmarks, run `go test -bench=.`.

Below are the results:

Validation Benchmarks (with cache)
------------

| Test                                       | Library      | Operations  | Time per op (ns) | Bytes per op | Allocs per op |
|--------------------------------------------|--------------|-------------|------------------|--------------|---------------|
| **BenchmarkValidateSimpleStruct**          | **firevault**| **839,338** | **1,443**        | **379**      | **5**         |
| *BenchmarkValidateSimpleStruct*            | *validator*  | *1,349,391* | *906.2*          | *0*          | *0*           |
| **BenchmarkValidateWithCustomRules**       | **firevault**| **743,604** | **1,524**        | **411**      | **7**         |
| *BenchmarkValidateWithCustomRules*         | *validator*  | *N/A*       | *N/A*            | *N/A*        | *N/A*         |
| **BenchmarkValidateStructWithSlice**       | **firevault**| **747,318** | **1,575**        | **403**      | **6**         |
| *BenchmarkValidateStructWithSlice*         | *validator*  | *1,000,000* | *1,060*          | *0*          | *0*           |
| **BenchmarkValidateNestedStructWithSlice** | **firevault**| **549,186** | **2,328**        | **793**      | **11**        |
| *BenchmarkValidateNestedStructWithSlice*   | *validator*  | *1,086,962* | *1,206*          | *0*          | *0*           |
| **BenchmarkValidateSliceOfSimpleStructs**  | **firevault**| **277,375** | **4,402**        | **1,137**    | **15**        |
| *BenchmarkValidateSliceOfSimpleStructs*    | *validator*  | *386,390*   | *3,297*          | *0*          | *0*           |

*The difference in performance and memory consumption can be attributed to the fact that Firevault extracts the validated data and creates a `map` from its values (which then gets passed to Firestore), as well as the fact that it supports transformations. Without this functionality, results are much closer.*

Validation Benchmarks (without cache)
------------

| Test                                       | Library      | Operations  | Time per op (ns) | Bytes per op | Allocs per op |
|--------------------------------------------|--------------|-------------|------------------|--------------|---------------|
| **BenchmarkValidateSimpleStruct**          | **firevault**| **276,624** | **4,308**        | **988**      | **26**        |
| *BenchmarkValidateSimpleStruct*            | *validator*  | *320,713*   | *4,053*          | *1,455*      | *34*          |
| **BenchmarkValidateWithCustomRules**       | **firevault**| **246,435** | **4,654**        | **1,069**    | **28**        |
| *BenchmarkValidateWithCustomRules*         | *validator*  | *N/A*       | *N/A*            | *N/A*        | *N/A*         |
| **BenchmarkValidateStructWithSlice**       | **firevault**| **224,590** | **5,285**        | **1,195**    | **34**        |
| *BenchmarkValidateStructWithSlice*         | *validator*  | *246,168*   | *4,949*          | *1,853*      | *43*          |
| **BenchmarkValidateNestedStructWithSlice** | **firevault**| **112,522** | **9,993**        | **2,228**    | **73**        |
| *BenchmarkValidateNestedStructWithSlice*   | *validator*  | *167,886*   | *7,689*          | *2,962*      | *72*          |
| **BenchmarkValidateSliceOfSimpleStructs**  | **firevault**| **94,486**  | **13,154**       | **2,966**    | **78**        |
| *BenchmarkValidateSliceOfSimpleStructs*    | *validator*  | *96,126*    | *12,654*         | *4,369*      | *102*         |