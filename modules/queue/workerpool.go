// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// WorkerPool takes
type WorkerPool struct {
	lock            sync.Mutex
	baseCtx         context.Context
	cancel          context.CancelFunc
	cond            *sync.Cond
	numberOfWorkers int
	batchLength     int
	handle          HandlerFunc
	dataChan        chan Data
	blockTimeout    time.Duration
	boostTimeout    time.Duration
	boostWorkers    int
}

// Push pushes the data to the internal channel
func (p *WorkerPool) Push(data Data) {
	p.lock.Lock()
	if p.blockTimeout > 0 && p.boostTimeout > 0 {
		p.lock.Unlock()
		p.pushBoost(data)
	} else {
		p.lock.Unlock()
		p.dataChan <- data
	}
}

func (p *WorkerPool) pushBoost(data Data) {
	select {
	case p.dataChan <- data:
	default:
		p.lock.Lock()
		if p.blockTimeout <= 0 {
			p.lock.Unlock()
			p.dataChan <- data
			return
		}
		ourTimeout := p.blockTimeout
		timer := time.NewTimer(p.blockTimeout)
		p.lock.Unlock()
		select {
		case p.dataChan <- data:
			if timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			p.lock.Lock()
			if p.blockTimeout > ourTimeout {
				p.lock.Unlock()
				p.dataChan <- data
				return
			}
			p.blockTimeout *= 2
			log.Warn("Worker Channel blocked for %v - adding %d temporary workers for %s, block timeout now %v", ourTimeout, p.boostWorkers, p.boostTimeout, p.blockTimeout)
			ctx, cancel := context.WithCancel(p.baseCtx)
			go func() {
				<-time.After(p.boostTimeout)
				cancel()
				p.lock.Lock()
				p.blockTimeout /= 2
				p.lock.Unlock()
			}()
			p.addWorkers(ctx, p.boostWorkers)
			p.lock.Unlock()
			p.dataChan <- data
		}
	}
}

// NumberOfWorkers returns the number of current workers in the pool
func (p *WorkerPool) NumberOfWorkers() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.numberOfWorkers
}

// AddWorkers adds workers to the pool
func (p *WorkerPool) AddWorkers(number int, timeout time.Duration) context.CancelFunc {
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(p.baseCtx, timeout)
	} else {
		ctx, cancel = context.WithCancel(p.baseCtx)
	}

	p.addWorkers(ctx, number)
	return cancel
}

// addWorkers adds workers to the pool
func (p *WorkerPool) addWorkers(ctx context.Context, number int) {
	for i := 0; i < number; i++ {
		p.lock.Lock()
		if p.cond == nil {
			p.cond = sync.NewCond(&p.lock)
		}
		p.numberOfWorkers++
		p.lock.Unlock()
		go func() {
			p.doWork(ctx)

			p.lock.Lock()
			p.numberOfWorkers--
			if p.numberOfWorkers <= 0 {
				// numberOfWorkers can't go negative but...
				p.numberOfWorkers = 0
				p.cond.Broadcast()
			}
			p.lock.Unlock()
		}()
	}
}

// Wait for WorkerPool to finish
func (p *WorkerPool) Wait() {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.cond == nil {
		p.cond = sync.NewCond(&p.lock)
	}
	if p.numberOfWorkers <= 0 {
		return
	}
	p.cond.Wait()
}

// CleanUp will drain the remaining contents of the channel
// This should be called after AddWorkers context is closed
func (p *WorkerPool) CleanUp(ctx context.Context) {
	log.Trace("CleanUp")
	close(p.dataChan)
	for data := range p.dataChan {
		p.handle(data)
		select {
		case <-ctx.Done():
			log.Warn("Cleanup context closed before finishing clean-up")
			return
		default:
		}
	}
	log.Trace("CleanUp done")
}

func (p *WorkerPool) doWork(ctx context.Context) {
	delay := time.Millisecond * 300
	var data = make([]Data, 0, p.batchLength)
	for {
		select {
		case <-ctx.Done():
			if len(data) > 0 {
				log.Trace("Handling: %d data, %v", len(data), data)
				p.handle(data...)
			}
			log.Trace("Worker shutting down")
			return
		case datum, ok := <-p.dataChan:
			if !ok {
				// the dataChan has been closed - we should finish up:
				if len(data) > 0 {
					log.Trace("Handling: %d data, %v", len(data), data)
					p.handle(data...)
				}
				log.Trace("Worker shutting down")
				return
			}
			data = append(data, datum)
			if len(data) >= p.batchLength {
				log.Trace("Handling: %d data, %v", len(data), data)
				p.handle(data...)
				data = make([]Data, 0, p.batchLength)
			}
		default:
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				if len(data) > 0 {
					log.Trace("Handling: %d data, %v", len(data), data)
					p.handle(data...)
				}
				log.Trace("Worker shutting down")
				return
			case datum, ok := <-p.dataChan:
				if timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				if !ok {
					// the dataChan has been closed - we should finish up:
					if len(data) > 0 {
						log.Trace("Handling: %d data, %v", len(data), data)
						p.handle(data...)
					}
					log.Trace("Worker shutting down")
					return
				}
				data = append(data, datum)
				if len(data) >= p.batchLength {
					log.Trace("Handling: %d data, %v", len(data), data)
					p.handle(data...)
					data = make([]Data, 0, p.batchLength)
				}
			case <-timer.C:
				delay = time.Millisecond * 100
				if len(data) > 0 {
					log.Trace("Handling: %d data, %v", len(data), data)
					p.handle(data...)
					data = make([]Data, 0, p.batchLength)
				}

			}
		}
	}
}
