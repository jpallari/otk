package validation

import (
	"fmt"
	"strconv"
	"strings"
)

type V struct {
	id     int
	name   string
	subs   []*V
	faults []fault
	depth  int
	stats  *stats
}

type stats struct {
	charCount  int
	faultCount int
	subCount   int
}

type fault struct {
	name        string
	description string
}

func (v *V) Init() {
	v.id = 0
	v.subs = nil
	v.faults = nil
	v.depth = 0
	v.stats = &stats{
		charCount:  0,
		faultCount: 0,
		subCount:   1,
	}
}

///////////////////////////////////
// Adding faults
///////////////////////////////////

func (v *V) Fail(name, description string) {
	if v.faults == nil {
		v.faults = make([]fault, 0, 10)
	}
	v.faults = append(v.faults, fault{
		name:        name,
		description: description,
	})
	v.stats.faultCount += 1
	v.stats.charCount += len(name) + len(description) + 2 + 2*v.depth
}

func (v *V) IndexFailF(index int, descriptionFormat string, a ...any) {
	v.FailF(strconv.Itoa(index), descriptionFormat, a...)
}

func (v *V) FailF(name, descriptionFormat string, a ...any) {
	description := fmt.Sprintf(descriptionFormat, a...)
	v.Fail(name, description)
}

func (v *V) IndexFail(index int, description string) {
	v.Fail(strconv.Itoa(index), description)
}

func (v *V) IndexFailFWhen(
	condition bool,
	index int,
	descriptionFormat string,
	a ...any,
) {
	if condition {
		v.FailF(strconv.Itoa(index), descriptionFormat, a...)
	}
}

func (v *V) FailFWhen(
	condition bool,
	name,
	descriptionFormat string,
	a ...any,
) {
	if condition {
		v.FailF(name, descriptionFormat, a...)
	}
}

func (v *V) IndexFailWhen(condition bool, index int, description string) {
	if condition {
		v.Fail(strconv.Itoa(index), description)
	}
}

func (v *V) FailWhen(condition bool, name, description string) {
	if condition {
		v.Fail(name, description)
	}
}

///////////////////////////////////
// Sub validator
///////////////////////////////////

func (v *V) Sub(name string) (child *V) {
	if v.subs == nil {
		v.subs = make([]*V, 0, 10)
	}

	for i := range v.subs {
		if v.subs[i].name == name {
			return v.subs[i]
		}
	}

	child = &V{
		name:  name,
		id:    v.stats.subCount,
		stats: v.stats,
		depth: v.depth + 1,
	}
	v.subs = append(v.subs, child)
	v.stats.subCount += 1
	return
}

func (v *V) IndexedSub(index int) *V {
	return v.Sub(strconv.Itoa(index))
}

///////////////////////////////////
// Report
///////////////////////////////////

func (v *V) Count() int {
	return v.stats.faultCount
}
func (v *V) Report() string {
	if v.Count() == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(2 * 1024)

	subFaultCount := make([]int, v.stats.subCount)
	var stack Stack
	{
		stack.Init(v.stats.subCount)
		stackData := make([]struct {
			visited bool
			parent  *V
			v       *V
		}, v.stats.subCount)

		stack.Push(0)
		stackData[0].v = v

		for {
			id, hasNext := stack.Pop()
			if !hasNext {
				break
			}
			parent := stackData[id].parent
			sv := stackData[id].v

			if stackData[id].visited {
				subFaultCount[id] += len(sv.faults)
				if parent != nil {
					subFaultCount[parent.id] += subFaultCount[sv.id]
				}
			} else {
				stackData[id].visited = true
				stack.Push(id)
				for i := range sv.subs {
					sub := sv.subs[i]
					if !stackData[sub.id].visited {
						stackData[sub.id].parent = sv
						stackData[sub.id].v = sv.subs[i]
						stack.Push(sub.id)
					}
				}
			}
		}
	}

	{
		stack.Init(v.stats.subCount)
		stackData := make([]struct {
			v *V
		}, v.stats.subCount)

		stack.Push(0)
		stackData[0].v = v

		for {
			id, hasNext := stack.Pop()
			if !hasNext {
				break
			}

			faultCount := subFaultCount[id]
			if faultCount == 0 {
				continue
			}

			sv := stackData[id].v

			if sv.depth > 0 {
				indent(&b, sv.depth-1)
				_, _ = b.WriteString(sv.name)
				_, _ = b.WriteString(":\n")
			}

			for _, fault := range sv.faults {
				indent(&b, sv.depth)
				_, _ = b.WriteString(fault.name)
				_, _ = b.WriteString(": ")
				_, _ = b.WriteString(fault.description)
				_ = b.WriteByte('\n')
			}

			for i := range sv.subs {
				index := len(sv.subs) - 1 - i
				sub := sv.subs[index]
				stack.Push(sub.id)
				stackData[sub.id].v = sub
			}
		}
	}

	return b.String()
}

func indent(b *strings.Builder, n int) {
	for range n * 2 {
		_ = b.WriteByte(' ')
	}
}

///////////////////////////////////
// Errors
///////////////////////////////////

func (v *V) ToError() error {
	if v.Count() <= 0 {
		return nil
	}
	return &ValidationError{
		report: v.Report(),
	}
}

type ValidationError struct {
	report string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed:\n%s", e.report)
}
