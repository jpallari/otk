package validation

import (
	"strings"
)

type Validator struct {
	id     int
	depth  int
	groups *[]group
	faults *[]fault
}

type group struct {
	parent int
	depth  int
	name   string
}

type fault struct {
	group       int
	name        string
	description string
}

func (this *Validator) Init(groups int, faults int) {
	gs := make([]group, 0, groups)
	fs := make([]fault, 0, faults)
	this.id = 0
	this.depth = 0
	this.groups = &gs
	this.faults = &fs
	*this.groups = append(*this.groups, group{
		parent: -1,
		depth:  0,
		name:   "",
	})
}

func New(groups int, faults int) Validator {
	validator := Validator{}
	validator.Init(groups, faults)
	return validator
}

func (this *Validator) AddFault(name string, description string) {
	*this.faults = append(*this.faults, fault{
		group:       this.id,
		name:        name,
		description: description,
	})
}

func (this *Validator) SubValidator(name string) Validator {
	id := len(*this.groups) + 1
	depth := this.depth + 1,
	*this.groups = append(*this.groups, group{
		parent: this.id,
		depth:  depth,
		name:   name,
	})
	return Validator{
		id:     id,
		depth:  depth,
		groups: this.groups,
		faults: this.faults,
	}
}

func (this *Validator) Count() int {
	return len(*this.faults)
}

func (this *Validator) String() string {
	var b strings.Builder
	b.Grow(this.Count() * 200)

	// Create a stack
	// Push first group to stack
	// while stack has elements:
	// - print faults for group
	// - find all groups with current group as parent (reversed)
	// - push the groups to stack

	// sort.Sort(faults(*this.faults))
	//
	// lastGroup := 0
	// lastGroupDepth := 0
	// for _, fault := range *this.faults {
	// 	depth := fault.depth
	// 	if lastGroup < fault.group {
	// 		for n := range fault.group - lastGroup - 1 {
	// 			groupId := lastGroup + n + 1
	// 			groupName := (*this.groups)[groupId].name
	// 			indent(&b, lastGroupDepth+n)
	// 			_, _ = b.WriteString(groupName)
	// 			_, _ = b.WriteString(":\n")
	// 		}
	// 	}
	// 	if depth > 0 {
	// 		// indent each element in group
	// 		// but only when group is not the root group
	// 		depth += 1
	// 	}
	// 	indent(&b, fault.depth)
	// 	_, _ = b.WriteString(fault.name)
	// 	_, _ = b.WriteString(": ")
	// 	_, _ = b.WriteString(fault.description)
	// 	_ = b.WriteByte('\n')
	// 	lastGroup = fault.group
	// 	lastGroupDepth = fault.depth
	// }

	return b.String()
}

func indent(b *strings.Builder, n int) {
	for range n * 2 {
		_ = b.WriteByte(' ')
	}
}

type faults []fault

func (this faults) Len() int {
	return len(this)
}

func (this faults) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

func (this faults) Less(i, j int) bool {
	itemi := this[i]
	itemj := this[j]

	if itemi.group == itemj.group {
		return itemi.name < itemj.name
	}
	return itemi.group < itemj.group
}
