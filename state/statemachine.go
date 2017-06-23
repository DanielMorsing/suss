package state

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/DanielMorsing/suss"
)

type Transition struct {
}

type Assert struct {
	Runner *suss.Runner
}

type Print struct{}

type StateMachine struct {
	runner          *suss.Runner
	machine         reflect.Value
	transitionStrs  []string
	transitionFuncs map[string]reflect.Value
	assertFuncs     map[string]reflect.Value
	printFunc       reflect.Value
}

func (s *StateMachine) Fill(d suss.Data) {
	var i int
	// TODO: actually care about edges
	sli := suss.Slice(func() {
		var u suss.Int63nGen
		u.N = int64(len(s.transitionStrs))
		s.runner.Draw(&u)
		tName := s.transitionStrs[u.Value]
		f := s.transitionFuncs[tName]
		t := reflect.ValueOf(new(Transition))
		f.Call([]reflect.Value{t})
		i++
		fmt.Printf("step %v: %v\n", i, tName)
		// alright, we did our transition
		// now run the printFunc
		p := []reflect.Value{reflect.ValueOf(new(Print))}
		if s.printFunc.IsValid() {
			s.printFunc.Call(p)
		}
		as := &Assert{
			Runner: s.runner,
		}
		for _, f := range s.assertFuncs {
			f.Call([]reflect.Value{reflect.ValueOf(as)})
		}
	})
	s.runner.Draw(sli)
}

var (
	transitionType = reflect.TypeOf((*Transition)(nil))
	assertType     = reflect.TypeOf((*Assert)(nil))
	printType      = reflect.TypeOf((*Print)(nil))
)

func Machine(r *suss.Runner, machine interface{}) *StateMachine {
	s := &StateMachine{
		runner:          r,
		transitionFuncs: make(map[string]reflect.Value),
		assertFuncs:     make(map[string]reflect.Value),
	}
	val := reflect.ValueOf(machine)
	nummethods := val.NumMethod()
	for i := 0; i < nummethods; i++ {
		meth := val.Method(i)
		methName := val.Type().Method(i).Name
		tmeth := meth.Type()
		numIn := tmeth.NumIn()
		if numIn != 1 {
			continue
		}
		in := tmeth.In(0)
		switch in {
		case transitionType:
			s.transitionStrs = append(s.transitionStrs, methName)
			s.transitionFuncs[methName] = meth
		case assertType:
			s.assertFuncs[methName] = meth
		case printType:
			if s.printFunc.IsValid() {
				panic("multiple print funcs")
			}
			s.printFunc = meth
		}
	}
	sort.Strings(s.transitionStrs)
	return s
}
