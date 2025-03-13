package validation

type Stack struct {
	nextIndex int
	elements  []int
}

func (this *Stack) Init(size int) {
	this.nextIndex = 0
	this.elements = make([]int, size)
}

func (this *Stack) Push(i int) {
	this.elements[this.nextIndex] = i
	this.nextIndex += 1
}

func (this *Stack) Pop() (int, bool) {
	if this.nextIndex <= 0 {
		return -1, false
	}
	this.nextIndex -= 1
	return this.elements[this.nextIndex], true
}
