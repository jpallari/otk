package validation

type Stack struct {
	nextIndex int
	elements  []int
}

func (s *Stack) InitWithBuf(buf []int) {
	s.nextIndex = 0
	s.elements = buf
}

func (s *Stack) Init(capacity int) {
	s.nextIndex = 0
	s.elements = make([]int, capacity)
}

func (s *Stack) Push(i int) bool {
	if s.nextIndex >= cap(s.elements) {
		return false
	}
	s.elements[s.nextIndex] = i
	s.nextIndex += 1
	return true
}

func (s *Stack) Pop() (int, bool) {
	if s.nextIndex <= 0 {
		return 0, false
	}
	s.nextIndex -= 1
	return s.elements[s.nextIndex], true
}
