package transform

import (
	"context"
)

// Manager is queue manager of transform items (Transform)
type Manager struct {
}

type provider func(ctx context.Context) *Transform

func (fc *Manager) runQueue(ctx context.Context, provider provider, concurrency int) <-chan *Transform {
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan bool, concurrency)
	outch := make(chan *Transform)
	run := true
	for run {
		sem <- true
		//fetch next transform
		//TODO check context done?
		t := provider(ctx)
		if t.IsComplete() {
			run = false
		} else {
			//waite till transform complite
			go func(t *Transform) {
				defer func() { <-sem }()
				t.Wait()
			}(t)
		}
	}
	//waite started transforms
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}
	return outch
}
