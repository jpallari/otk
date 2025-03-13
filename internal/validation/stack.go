package validation

type Stack struct {
	nextIndex int
	elements  []int
}

func (this *Stack) InitWithBuf(buf []int) {
	this.nextIndex = 0
	this.elements = buf
}

func (this *Stack) Init(capacity int) {
	this.nextIndex = 0
	this.elements = make([]int, capacity)
}

func (this *Stack) Push(i int) bool {
	if this.nextIndex >= cap(this.elements) {
		return false
	}
	this.elements[this.nextIndex] = i
	this.nextIndex += 1
	return true
}

func (this *Stack) Pop() (int, bool) {
	if this.nextIndex <= 0 {
		return 0, false
	}
	this.nextIndex -= 1
	return this.elements[this.nextIndex], true
}
