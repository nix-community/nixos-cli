package set

func Unique[T comparable](in []T) []T {
	set := make(map[T]struct{}, len(in))

	for _, v := range in {
		set[v] = struct{}{}
	}

	out := make([]T, 0, len(set))
	for v := range set {
		out = append(out, v)
	}

	return out
}
