package sort

import (
	"sort"
	"testing"

	"github.com/DanielMorsing/suss"
)

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

}

func sortFloats(f []float64) {
	sort.Slice(f, func(i, j int) bool {
		return f[i] < f[j]
	})
}
