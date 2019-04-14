package pool

type SingleConnPool struct {
	cn *Conn
}

var _ Pooler = (*SingleConnPool)(nil)

func NewSingleConnPool(cn *Conn) *SingleConnPool {
	return &SingleConnPool{
		cn: cn,
	}
}

func (p *SingleConnPool) NewConn() (*Conn, error) {
	panic("not implemented")
}

func (p *SingleConnPool) CloseConn(*Conn) error {
	panic("not implemented")
}

func (p *SingleConnPool) Get() (*Conn, error) {
	return p.cn, nil
}

func (p *SingleConnPool) Put(cn *Conn) {
	if p.cn != cn {
		panic("p.cn != cn")
	}
}

func (p *SingleConnPool) Remove(cn *Conn) {
	if p.cn != cn {
		panic("p.cn != cn")
	}
}

func (p *SingleConnPool) Len() int {
	return 1
}

func (p *SingleConnPool) IdleLen() int {
	return 0
}

func (p *SingleConnPool) Stats() *Stats {
	return nil
}

func (p *SingleConnPool) Close() error {
	return nil
}
