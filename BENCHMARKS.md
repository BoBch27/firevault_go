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

| Test                                       | Library      | Operations  | Time per op (ns) | Bytes per op | Allocs per op |
|--------------------------------------------|--------------|-------------|------------------|--------------|---------------|
| **BenchmarkValidateSimpleStruct**          | **firevault**| **701,134** | **1,586**        | **380**      | **5**         |
| *BenchmarkValidateSimpleStruct*            | *validator*  | *1,349,391* | *906.2*          | *0*          | *0*           |
| **BenchmarkValidateWithCustomRules**       | **firevault**| **637,363** | **1,659**        | **411**      | **7**         |
| *BenchmarkValidateWithCustomRules*         | *validator*  | *N/A*       | *N/A*            | *N/A*        | *N/A*         |
| **BenchmarkValidateStructWithSlice**       | **firevault**| **642,999** | **1,766**        | **404**      | **6**         |
| *BenchmarkValidateStructWithSlice*         | *validator*  | *1,000,000* | *1,060*          | *0*          | *0*           |
| **BenchmarkValidateNestedStructWithSlice** | **firevault**| **427,381** | **2,552**        | **793**      | **11**        |
| *BenchmarkValidateNestedStructWithSlice*   | *validator*  | *1,086,962* | *1,206*          | *0*          | *0*           |
| **BenchmarkValidateSliceOfSimpleStructs**  | **firevault**| **242,559** | **4,933**        | **1,140**    | **15**        |
| *BenchmarkValidateSliceOfSimpleStructs*    | *validator*  | *386,390*   | *3,297*          | *0*          | *0*           |

*The difference in performance and memory consumption can be attributed to the fact that Firevault extracts the validated data and creates a `map` from its values (which then gets passed to Firestore), as well as the fact that it supports transformations. Without this functionality, results are much closer.*

Validation Benchmarks (without cache)
------------

| Test                                       | Library      | Operations  | Time per op (ns) | Bytes per op | Allocs per op |
|--------------------------------------------|--------------|-------------|------------------|--------------|---------------|
| **BenchmarkValidateSimpleStruct**          | **firevault**| **270,284** | **4,627**        | **989**      | **26**        |
| *BenchmarkValidateSimpleStruct*            | *validator*  | *264,194*   | *4,708*          | *1,716*      | *36*          |
| **BenchmarkValidateWithCustomRules**       | **firevault**| **237,027** | **4,851**        | **1,069**    | **28**        |
| *BenchmarkValidateWithCustomRules*         | *validator*  | *N/A*       | *N/A*            | *N/A*        | *N/A*         |
| **BenchmarkValidateStructWithSlice**       | **firevault**| **207,734** | **5,631**        | **1,198**    | **34**        |
| *BenchmarkValidateStructWithSlice*         | *validator*  | *218,505*   | *5,401*          | *2,110*      | *45*          |
| **BenchmarkValidateNestedStructWithSlice** | **firevault**| **111,862** | **9,493**        | **2,235**    | **73**        |
| *BenchmarkValidateNestedStructWithSlice*   | *validator*  | *122,714*   | *8,987*          | *3,485*      | *76*          |
| **BenchmarkValidateSliceOfSimpleStructs**  | **firevault**| **92,987**  | **14,263**       | **2,973**    | **78**        |
| *BenchmarkValidateSliceOfSimpleStructs*    | *validator*  | *91,512*    | *14,358*         | *14,358*     | *108*         |
