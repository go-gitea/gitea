package pool

import (
	"fmt"
	"sync/atomic"
)

const (
	stateDefault = 0
	stateInited  = 1
	stateClosed  = 2
)

type BadConnError struct {
	wrapped error
}

var _ error = (*BadConnError)(nil)

func (e BadConnError) Error() string {
	return "pg: Conn is in a bad state"
}

func (e BadConnError) Unwrap() error {
	return e.wrapped
}

type SingleConnPool struct {
	pool  Pooler
	level int32 // atomic

	state uint32 // atomic
	ch    chan *Conn

	_badConnError atomic.Value
}

var _ Pooler = (*SingleConnPool)(nil)

func NewSingleConnPool(pool Pooler) *SingleConnPool {
	p, ok := pool.(*SingleConnPool)
	if !ok {
		p = &SingleConnPool{
			pool: pool,
			ch:   make(chan *Conn, 1),
		}
	}
	atomic.AddInt32(&p.level, 1)
	return p
}

func (p *SingleConnPool) SetConn(cn *Conn) {
	if atomic.CompareAndSwapUint32(&p.state, stateDefault, stateInited) {
		p.ch <- cn
	} else {
		panic("not reached")
	}
}

func (p *SingleConnPool) NewConn() (*Conn, error) {
	return p.pool.NewConn()
}

func (p *SingleConnPool) CloseConn(cn *Conn) error {
	return p.pool.CloseConn(cn)
}

func (p *SingleConnPool) Get() (*Conn, error) {
	// In worst case this races with Close which is not a very common operation.
	for i := 0; i < 1000; i++ {
		switch atomic.LoadUint32(&p.state) {
		case stateDefault:
			cn, err := p.pool.Get()
			if err != nil {
				return nil, err
			}
			if atomic.CompareAndSwapUint32(&p.state, stateDefault, stateInited) {
				return cn, nil
			}
			p.pool.Remove(cn, ErrClosed)
		case stateInited:
			if err := p.badConnError(); err != nil {
				return nil, err
			}
			cn, ok := <-p.ch
			if !ok {
				return nil, ErrClosed
			}
			return cn, nil
		case stateClosed:
			return nil, ErrClosed
		default:
			panic("not reached")
		}
	}
	return nil, fmt.Errorf("pg: SingleConnPool.Get: infinite loop")
}

func (p *SingleConnPool) Put(cn *Conn) {
	defer func() {
		if recover() != nil {
			p.freeConn(cn)
		}
	}()
	p.ch <- cn
}

func (p *SingleConnPool) freeConn(cn *Conn) {
	if err := p.badConnError(); err != nil {
		p.pool.Remove(cn, err)
	} else {
		p.pool.Put(cn)
	}
}

func (p *SingleConnPool) Remove(cn *Conn, reason error) {
	defer func() {
		if recover() != nil {
			p.pool.Remove(cn, ErrClosed)
		}
	}()
	p._badConnError.Store(BadConnError{wrapped: reason})
	p.ch <- cn
}

func (p *SingleConnPool) Len() int {
	switch atomic.LoadUint32(&p.state) {
	case stateDefault:
		return 0
	case stateInited:
		return 1
	case stateClosed:
		return 0
	default:
		panic("not reached")
	}
}

func (p *SingleConnPool) IdleLen() int {
	return len(p.ch)
}

func (p *SingleConnPool) Stats() *Stats {
	return &Stats{}
}

func (p *SingleConnPool) Close() error {
	level := atomic.AddInt32(&p.level, -1)
	if level > 0 {
		return nil
	}

	for i := 0; i < 1000; i++ {
		state := atomic.LoadUint32(&p.state)
		if state == stateClosed {
			return ErrClosed
		}
		if atomic.CompareAndSwapUint32(&p.state, state, stateClosed) {
			close(p.ch)
			cn, ok := <-p.ch
			if ok {
				p.freeConn(cn)
			}
			return nil
		}
	}

	return fmt.Errorf("pg: SingleConnPool.Close: infinite loop")
}

func (p *SingleConnPool) Reset() error {
	if p.badConnError() == nil {
		return nil
	}

	select {
	case cn, ok := <-p.ch:
		if !ok {
			return ErrClosed
		}
		p.pool.Remove(cn, ErrClosed)
		p._badConnError.Store(BadConnError{wrapped: nil})
	default:
		return fmt.Errorf("pg: SingleConnPool does not have a Conn")
	}

	if !atomic.CompareAndSwapUint32(&p.state, stateInited, stateDefault) {
		state := atomic.LoadUint32(&p.state)
		return fmt.Errorf("pg: invalid SingleConnPool state: %d", state)
	}

	return nil
}

func (p *SingleConnPool) badConnError() error {
	if v := p._badConnError.Load(); v != nil {
		err := v.(BadConnError)
		if err.wrapped != nil {
			return err
		}
	}
	return nil
}
