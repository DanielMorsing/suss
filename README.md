Suspicion

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
INSERT EXAMPLE HERE
```

Suspicion not only finds a counter example, but also tries to find a minimal example.

Suspicion is heavily influenced by the python library Hypothesis. [http://hypothesis.works/](Their website) has a lot of useful information on what property-based testing is and how to use it effectively.

