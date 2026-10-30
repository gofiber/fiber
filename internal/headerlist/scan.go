package headerlist

// Next returns the raw comma-separated element starting at start and the next
// start position. The element aliases list; whitespace and empty elements are
// preserved. Every comma separates, including commas inside quotes.
//
// start must be between zero and len(list), inclusive. The returned position
// exceeds len(list) after the final element, allowing callers to stop early
// without allocating a slice of all elements.
func Next(list string, start int) (element string, next int) { //nolint:nonamedreturns // names distinguish the element from the next position
	end := start
	for end < len(list) && list[end] != ',' {
		end++
	}
	return list[start:end], end + 1
}

// Prev is the reverse of Next. It returns the raw element ending just before
// end and the next end position. end must be between zero and len(list),
// inclusive. The returned position is negative after the first element.
func Prev(list string, end int) (element string, prev int) { //nolint:nonamedreturns // names distinguish the element from the previous position
	start := end
	for start > 0 && list[start-1] != ',' {
		start--
	}
	return list[start:end], start - 1
}
