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

func (this *V) Init() {
	this.id = 0
	this.subs = nil
	this.faults = nil
	this.depth = 0
	this.stats = &stats{
		charCount:  0,
		faultCount: 0,
		subCount:   1,
	}
}

///////////////////////////////////
// Adding faults
///////////////////////////////////

func (this *V) Fail(name, description string) {
	if this.faults == nil {
		this.faults = make([]fault, 0, 10)
	}
	this.faults = append(this.faults, fault{
		name:        name,
		description: description,
	})
	this.stats.faultCount += 1
	this.stats.charCount += len(name) + len(description) + 2 + 2*this.depth
}

func (this *V) IndexFailF(index int, descriptionFormat string, a ...any) {
	this.FailF(strconv.Itoa(index), descriptionFormat, a...)
}

func (this *V) FailF(name, descriptionFormat string, a ...any) {
	description := fmt.Sprintf(descriptionFormat, a...)
	this.Fail(name, description)
}

func (this *V) IndexFail(index int, description string) {
	this.Fail(strconv.Itoa(index), description)
}

func (this *V) IndexFailFWhen(
	condition bool,
	index int,
	descriptionFormat string,
	a ...any,
) {
	if condition {
		this.FailF(strconv.Itoa(index), descriptionFormat, a...)
	}
}

func (this *V) FailFWhen(
	condition bool,
	name,
	descriptionFormat string,
	a ...any,
) {
	if condition {
		this.FailF(name, descriptionFormat, a...)
	}
}

func (this *V) IndexFailWhen(condition bool, index int, description string) {
	if condition {
		this.Fail(strconv.Itoa(index), description)
	}
}

func (this *V) FailWhen(condition bool, name, description string) {
	if condition {
		this.Fail(name, description)
	}
}

///////////////////////////////////
// Sub validator
///////////////////////////////////

func (this *V) Sub(name string) (child *V) {
	if this.subs == nil {
		this.subs = make([]*V, 0, 10)
	}

	for i := range this.subs {
		if this.subs[i].name == name {
			return this.subs[i]
		}
	}

	child = &V{
		name:  name,
		id:    this.stats.subCount,
		stats: this.stats,
		depth: this.depth + 1,
	}
	this.subs = append(this.subs, child)
	this.stats.subCount += 1
	return
}

func (this *V) IndexedSub(index int) *V {
	return this.Sub(strconv.Itoa(index))
}

///////////////////////////////////
// Report
///////////////////////////////////

func (this *V) Count() int {
	return this.stats.faultCount
}
func (this *V) Report() string {
	if this.Count() == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(2 * 1024)

	subFaultCount := make([]int, this.stats.subCount)
	var stack Stack
	{
		stack.Init(this.stats.subCount)
		stackData := make([]struct {
			visited bool
			parent  *V
			v       *V
		}, this.stats.subCount)

		stack.Push(0)
		stackData[0].v = this

		for {
			id, hasNext := stack.Pop()
			if !hasNext {
				break
			}
			parent := stackData[id].parent
			v := stackData[id].v

			if stackData[id].visited {
				subFaultCount[id] += len(v.faults)
				if parent != nil {
					subFaultCount[parent.id] += subFaultCount[v.id]
				}
			} else {
				stackData[id].visited = true
				stack.Push(id)
				for i := range v.subs {
					sub := v.subs[i]
					if !stackData[sub.id].visited {
						stackData[sub.id].parent = v
						stackData[sub.id].v = v.subs[i]
						stack.Push(sub.id)
					}
				}
			}
		}
	}

	{
		stack.Init(this.stats.subCount)
		stackData := make([]struct {
			v *V
		}, this.stats.subCount)

		stack.Push(0)
		stackData[0].v = this

		for {
			id, hasNext := stack.Pop()
			if !hasNext {
				break
			}

			faultCount := subFaultCount[id]
			if faultCount == 0 {
				continue
			}

			v := stackData[id].v

			if v.depth > 0 {
				indent(&b, v.depth-1)
				_, _ = b.WriteString(v.name)
				_, _ = b.WriteString(":\n")
			}

			for _, fault := range v.faults {
				indent(&b, v.depth)
				_, _ = b.WriteString(fault.name)
				_, _ = b.WriteString(": ")
				_, _ = b.WriteString(fault.description)
				_ = b.WriteByte('\n')
			}

			for i := range v.subs {
				index := len(v.subs) - 1 - i
				sub := v.subs[index]
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

func (this *V) ToError() error {
	if this.Count() <= 0 {
		return nil
	}
	return &ValidationError{
		report: this.Report(),
	}
}

type ValidationError struct {
	report string
}

func (this *ValidationError) Error() string {
	return fmt.Sprintf("validation failed:\n%s", this.report)
}
