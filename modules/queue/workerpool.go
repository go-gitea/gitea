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
	lock               sync.Mutex
	baseCtx            context.Context
	cancel             context.CancelFunc
	cond               *sync.Cond
	qid                int64
	maxNumberOfWorkers int
	numberOfWorkers    int
	batchLength        int
	handle             HandlerFunc
	dataChan           chan Data
	blockTimeout       time.Duration
	boostTimeout       time.Duration
	boostWorkers       int
}

// Push pushes the data to the internal channel
func (p *WorkerPool) Push(data Data) {
	p.lock.Lock()
	if p.blockTimeout > 0 && p.boostTimeout > 0 && (p.numberOfWorkers <= p.maxNumberOfWorkers || p.maxNumberOfWorkers < 0) {
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
			if p.blockTimeout > ourTimeout || (p.numberOfWorkers > p.maxNumberOfWorkers && p.maxNumberOfWorkers >= 0) {
				p.lock.Unlock()
				p.dataChan <- data
				return
			}
			p.blockTimeout *= 2
			ctx, cancel := context.WithCancel(p.baseCtx)
			mq := GetManager().GetManagedQueue(p.qid)
			boost := p.boostWorkers
			if (boost+p.numberOfWorkers) > p.maxNumberOfWorkers && p.maxNumberOfWorkers >= 0 {
				boost = p.maxNumberOfWorkers - p.numberOfWorkers
			}
			if mq != nil {
				log.Warn("WorkerPool: %d (for %s) Channel blocked for %v - adding %d temporary workers for %s, block timeout now %v", p.qid, mq.Name, ourTimeout, boost, p.boostTimeout, p.blockTimeout)

				start := time.Now()
				pid := mq.RegisterWorkers(boost, start, false, start, cancel)
				go func() {
					<-ctx.Done()
					mq.RemoveWorkers(pid)
					cancel()
				}()
			} else {
				log.Warn("WorkerPool: %d Channel blocked for %v - adding %d temporary workers for %s, block timeout now %v", p.qid, ourTimeout, p.boostWorkers, p.boostTimeout, p.blockTimeout)
			}
			go func() {
				<-time.After(p.boostTimeout)
				cancel()
				p.lock.Lock()
				p.blockTimeout /= 2
				p.lock.Unlock()
			}()
			p.lock.Unlock()
			p.addWorkers(ctx, boost)
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

// MaxNumberOfWorkers returns the maximum number of workers automatically added to the pool
func (p *WorkerPool) MaxNumberOfWorkers() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.maxNumberOfWorkers
}

// BoostWorkers returns the number of workers for a boost
func (p *WorkerPool) BoostWorkers() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.boostWorkers
}

// BoostTimeout returns the timeout of the next boost
func (p *WorkerPool) BoostTimeout() time.Duration {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.boostTimeout
}

// BlockTimeout returns the timeout til the next boost
func (p *WorkerPool) BlockTimeout() time.Duration {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.blockTimeout
}

// SetSettings sets the setable boost values
func (p *WorkerPool) SetSettings(maxNumberOfWorkers, boostWorkers int, timeout time.Duration) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.maxNumberOfWorkers = maxNumberOfWorkers
	p.boostWorkers = boostWorkers
	p.boostTimeout = timeout
}

// SetMaxNumberOfWorkers sets the maximum number of workers automatically added to the pool
// Changing this number will not change the number of current workers but will change the limit
// for future additions
func (p *WorkerPool) SetMaxNumberOfWorkers(newMax int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.maxNumberOfWorkers = newMax
}

// AddWorkers adds workers to the pool - this allows the number of workers to go above the limit
func (p *WorkerPool) AddWorkers(number int, timeout time.Duration) context.CancelFunc {
	var ctx context.Context
	var cancel context.CancelFunc
	start := time.Now()
	end := start
	hasTimeout := false
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(p.baseCtx, timeout)
		end = start.Add(timeout)
		hasTimeout = true
	} else {
		ctx, cancel = context.WithCancel(p.baseCtx)
	}

	mq := GetManager().GetManagedQueue(p.qid)
	if mq != nil {
		pid := mq.RegisterWorkers(number, start, hasTimeout, end, cancel)
		go func() {
			<-ctx.Done()
			mq.RemoveWorkers(pid)
			cancel()
		}()
		log.Trace("WorkerPool: %d (for %s) adding %d workers with group id: %d", p.qid, mq.Name, number, pid)
	} else {
		log.Trace("WorkerPool: %d adding %d workers (no group id)", p.qid, number)

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
			if p.numberOfWorkers == 0 {
				p.cond.Broadcast()
			} else if p.numberOfWorkers < 0 {
				// numberOfWorkers can't go negative but...
				log.Warn("Number of Workers < 0 for QID %d - this shouldn't happen", p.qid)
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
	log.Trace("WorkerPool: %d CleanUp", p.qid)
	close(p.dataChan)
	for data := range p.dataChan {
		p.handle(data)
		select {
		case <-ctx.Done():
			log.Warn("WorkerPool: %d Cleanup context closed before finishing clean-up", p.qid)
			return
		default:
		}
	}
	log.Trace("WorkerPool: %d CleanUp Done", p.qid)
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
