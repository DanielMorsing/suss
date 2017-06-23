package diehard

import (
	"fmt"
	"testing"

	"github.com/DanielMorsing/suss"
	"github.com/DanielMorsing/suss/state"
)

type DieHard struct {
	small int
	large int
}

func (d *DieHard) FillSmall(s *state.Transition) {
	d.small = 3
}

func (d *DieHard) FillLarge(s *state.Transition) {
	d.large = 5
}

func (d *DieHard) EmptySmall(s *state.Transition) {
	d.small = 0
}

func (d *DieHard) EmptyLarge(s *state.Transition) {
	d.large = 0
}

func (d *DieHard) LargeIntoSmall(s *state.Transition) {
	roomInSmall := 3 - d.small
	if d.large < roomInSmall {
		d.small += d.large
		d.large = 0
	} else {
		d.large -= roomInSmall
		d.small = 3
	}
}

func (d *DieHard) SmallIntoLarge(s *state.Transition) {
	roomInLarge := 5 - d.large
	if d.small < roomInLarge {
		d.large += d.small
		d.small = 0
	} else {
		d.small -= roomInLarge
		d.large = 5
	}
}

func (d *DieHard) AssertLogic(s *state.Assert) {
	if d.small > 3 || d.large > 5 {
		s.Runner.Fatalf("d.small or d.large larger than possible")
	}
	if d.small < 0 || d.large < 0 {
		s.Runner.Fatalf("d.small or d.large less than empty")
	}
}

func (d *DieHard) AssertResult(s *state.Assert) {
	if d.large == 4 {
		s.Runner.Fatalf("found die hard result")
	}
}

func (d *DieHard) Print(s *state.Print) {
	fmt.Printf("d.small: %v. d.large: %v\n", d.small, d.large)
}

func TestDieHard(t *testing.T) {
	s := suss.NewTest(t)
	s.Run(func() {
		d := DieHard{}
		state := state.Machine(s, &d)
		s.Draw(state)
	})
}
