package coordinator

import (
	"fmt"
	"sync/atomic"
)

type stageRegistry struct {
	next atomic.Uint64
}

func (r *stageRegistry) Resolve(name string) string {
	if name != "" {
		return name
	}

	id := r.next.Add(1)
	return fmt.Sprintf("stage-%d", id)
}
