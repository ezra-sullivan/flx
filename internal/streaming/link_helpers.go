package streaming

// next receives one item from s while updating any attached link meter.
func (s Stream[T]) next() (T, bool) {
	item, ok := <-s.source
	if !ok {
		s.closeLink()
		var zero T
		return zero, false
	}

	s.observeLinkReceive()
	return item, true
}

func (s Stream[T]) observeLinkReceive() {
	if s.link == nil {
		return
	}

	s.link.ObserveReceive()
}

func (s Stream[T]) setLinkDownstreamStage(name string) {
	if s.link == nil {
		return
	}

	s.link.SetDownstreamStage(name)
}

func (s Stream[T]) closeLink() {
	if s.link == nil {
		return
	}

	s.link.Close()
}

func (s Stream[T]) drainSource() {
	for {
		if _, ok := s.next(); !ok {
			return
		}
	}
}

func (s Stream[T]) withObservedSource(fn func(<-chan T)) {
	if s.link == nil {
		fn(s.source)
		s.drainSource()
		return
	}

	proxy := make(chan T)
	done := make(chan struct{})
	drained := make(chan struct{})

	go func() {
		defer close(proxy)
		defer close(drained)

		forwarding := true
		for {
			item, ok := s.next()
			if !ok {
				return
			}

			if !forwarding {
				continue
			}

			select {
			case proxy <- item:
			case <-done:
				forwarding = false
			}
		}
	}()

	fn(proxy)
	close(done)
	<-drained
}

func withObservedSourceResult[T, R any](s Stream[T], fn func(<-chan T) (R, error)) (R, error) {
	if s.link == nil {
		result, err := fn(s.source)
		s.drainSource()
		return result, err
	}

	proxy := make(chan T)
	done := make(chan struct{})
	drained := make(chan struct{})

	go func() {
		defer close(proxy)
		defer close(drained)

		forwarding := true
		for {
			item, ok := s.next()
			if !ok {
				return
			}

			if !forwarding {
				continue
			}

			select {
			case proxy <- item:
			case <-done:
				forwarding = false
			}
		}
	}()

	result, err := fn(proxy)
	close(done)
	<-drained
	return result, err
}
