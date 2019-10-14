package pool

import "sync"

type StickyConnPool struct {
	pool     *ConnPool
	reusable bool

	cn     *Conn
	closed bool
	mu     sync.Mutex
}

var _ Pooler = (*StickyConnPool)(nil)

func NewStickyConnPool(pool *ConnPool, reusable bool) *StickyConnPool {
	return &StickyConnPool{
		pool:     pool,
		reusable: reusable,
	}
}

func (p *StickyConnPool) NewConn() (*Conn, error) {
	panic("not implemented")
}

func (p *StickyConnPool) CloseConn(*Conn) error {
	panic("not implemented")
}

func (p *StickyConnPool) Get() (*Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, ErrClosed
	}
	if p.cn != nil {
		return p.cn, nil
	}

	cn, err := p.pool.Get()
	if err != nil {
		return nil, err
	}

	p.cn = cn
	return cn, nil
}

func (p *StickyConnPool) putUpstream() {
	p.pool.Put(p.cn)
	p.cn = nil
}

func (p *StickyConnPool) Put(cn *Conn) {}

func (p *StickyConnPool) removeUpstream(reason error) {
	p.pool.Remove(p.cn, reason)
	p.cn = nil
}

func (p *StickyConnPool) Remove(cn *Conn, reason error) {
	p.removeUpstream(reason)
}

func (p *StickyConnPool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cn == nil {
		return 0
	}
	return 1
}

func (p *StickyConnPool) IdleLen() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cn == nil {
		return 1
	}
	return 0
}

func (p *StickyConnPool) Stats() *Stats {
	return nil
}

func (p *StickyConnPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrClosed
	}
	p.closed = true

	if p.cn != nil {
		if p.reusable {
			p.putUpstream()
		} else {
			p.removeUpstream(ErrClosed)
		}
	}

	return nil
}
