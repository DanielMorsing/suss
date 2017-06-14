Suspicion
----------
Suspicion is a property-based testing library for Go.

Property testing?
=================

Property-based testing uses random generation of data to find edge cases that violate some property.

An example is a simple sort function

```
func sortFloats(f []float64) {
	sort.Slice(f, func(i, j int) bool {
		return f[i] < f[j]
	})
}
```

After this function has been called and our implementation is correct, you can assume the property `f[i] <= f[i+1]`

With Suspicion, you can write a short test that verifies this property

```
func TestSort(t *testing.T) {
	s := suss.NewTest(t)
	s.Run(func() {
		var f []float64
		s.Slice().Gen(func() {
			n := s.Float64()
			f = append(f, n)
		})
		sortFloats(f)
		for i := 0; i < len(f)-1; i++ {
			if f[i] > f[i+1] {
				s.Fatalf("invalid sort len=%v, %v\n", len(f), f)
			}
		}
	})

```

Suspicion not only finds a counter example, but also tries to find a minimal example.

```
invalid sort len=13, [NaN 0 0 0 0 4.394777251413336e+230 0 0 0 0 0 0 0]
--- FAIL: TestSort (0.02s)
```

In this case, we find that a 13 length slice with a NaN and a non-zero value results in a invalid sort. Note that this condition doesn't fail with a 12 length array. Go uses a different sort algorithm for smaller slices and because of the float comparisons involved, it sorts NaNs at the end of the slice.

Suspicion is heavily influenced by the python library Hypothesis. [Their website](http://hypothesis.works/) has a lot of useful information on what property-based testing is and how to use it effectively.

