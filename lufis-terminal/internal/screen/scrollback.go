package screen

type ringBuffer struct {
	data      [][]Cell
	lineWidth int
	head      int
	count     int
	cap       int
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		data: make([][]Cell, capacity),
		cap:  capacity,
	}
}

func (r *ringBuffer) push(line []Cell, width int) {
	r.lineWidth = width
	lineCopy := make([]Cell, len(line))
	copy(lineCopy, line)

	if r.count < r.cap {
		idx := (r.head + r.count) % r.cap
		r.data[idx] = lineCopy
		r.count++
	} else {
		r.data[r.head] = lineCopy
		r.head = (r.head + 1) % r.cap
	}
}

func (r *ringBuffer) Lines() [][]Cell {
	result := make([][]Cell, r.count)
	for i := 0; i < r.count; i++ {
		idx := (r.head + i) % r.cap
		result[i] = r.data[idx]
	}
	return result
}

func (r *ringBuffer) Line(idx int) []Cell {
	if idx < 0 || idx >= r.count {
		return nil
	}
	realIdx := (r.head + idx) % r.cap
	return r.data[realIdx]
}

func (r *ringBuffer) Count() int { return r.count }

func (r *ringBuffer) Clear() {
	r.head = 0
	r.count = 0
}
