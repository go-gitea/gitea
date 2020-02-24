package couchbase

import (
	"fmt"
	"github.com/couchbase/goutils/logging"
	"sync"
)

type PersistTo uint8

const (
	PersistNone   = PersistTo(0x00)
	PersistMaster = PersistTo(0x01)
	PersistOne    = PersistTo(0x02)
	PersistTwo    = PersistTo(0x03)
	PersistThree  = PersistTo(0x04)
	PersistFour   = PersistTo(0x05)
)

type ObserveTo uint8

const (
	ObserveNone           = ObserveTo(0x00)
	ObserveReplicateOne   = ObserveTo(0x01)
	ObserveReplicateTwo   = ObserveTo(0x02)
	ObserveReplicateThree = ObserveTo(0x03)
	ObserveReplicateFour  = ObserveTo(0x04)
)

type JobType uint8

const (
	OBSERVE = JobType(0x00)
	PERSIST = JobType(0x01)
)

type ObservePersistJob struct {
	vb                 uint16
	vbuuid             uint64
	hostname           string
	jobType            JobType
	failover           uint8
	lastPersistedSeqNo uint64
	currentSeqNo       uint64
	resultChan         chan *ObservePersistJob
	errorChan          chan *OPErrResponse
}

type OPErrResponse struct {
	vb     uint16
	vbuuid uint64
	err    error
	job    *ObservePersistJob
}

var ObservePersistPool = NewPool(1024)
var OPJobChan = make(chan *ObservePersistJob, 1024)
var OPJobDone = make(chan bool)

var wg sync.WaitGroup

func (b *Bucket) StartOPPollers(maxWorkers int) {

	for i := 0; i < maxWorkers; i++ {
		go b.OPJobPoll()
		wg.Add(1)
	}
	wg.Wait()
}

func (b *Bucket) SetObserveAndPersist(nPersist PersistTo, nObserve ObserveTo) (err error) {

	numNodes := len(b.Nodes())
	if int(nPersist) > numNodes || int(nObserve) > numNodes {
		return fmt.Errorf("Not enough healthy nodes in the cluster")
	}

	if int(nPersist) > (b.Replicas+1) || int(nObserve) > b.Replicas {
		return fmt.Errorf("Not enough replicas in the cluster")
	}

	if EnableMutationToken == false {
		return fmt.Errorf("Mutation Tokens not enabled ")
	}

	b.ds = &DurablitySettings{Persist: PersistTo(nPersist), Observe: ObserveTo(nObserve)}
	return
}

func (b *Bucket) ObserveAndPersistPoll(vb uint16, vbuuid uint64, seqNo uint64) (err error, failover bool) {
	b.RLock()
	ds := b.ds
	b.RUnlock()

	if ds == nil {
		return
	}

	nj := 0 // total number of jobs
	resultChan := make(chan *ObservePersistJob, 10)
	errChan := make(chan *OPErrResponse, 10)

	nodes := b.GetNodeList(vb)
	if int(ds.Observe) > len(nodes) || int(ds.Persist) > len(nodes) {
		return fmt.Errorf("Not enough healthy nodes in the cluster"), false
	}

	logging.Infof("Node list %v", nodes)

	if ds.Observe >= ObserveReplicateOne {
		// create a job for each host
		for i := ObserveReplicateOne; i < ds.Observe+1; i++ {
			opJob := ObservePersistPool.Get()
			opJob.vb = vb
			opJob.vbuuid = vbuuid
			opJob.jobType = OBSERVE
			opJob.hostname = nodes[i]
			opJob.resultChan = resultChan
			opJob.errorChan = errChan

			OPJobChan <- opJob
			nj++

		}
	}

	if ds.Persist >= PersistMaster {
		for i := PersistMaster; i < ds.Persist+1; i++ {
			opJob := ObservePersistPool.Get()
			opJob.vb = vb
			opJob.vbuuid = vbuuid
			opJob.jobType = PERSIST
			opJob.hostname = nodes[i]
			opJob.resultChan = resultChan
			opJob.errorChan = errChan

			OPJobChan <- opJob
			nj++

		}
	}

	ok := true
	for ok {
		select {
		case res := <-resultChan:
			jobDone := false
			if res.failover == 0 {
				// no failover
				if res.jobType == PERSIST {
					if res.lastPersistedSeqNo >= seqNo {
						jobDone = true
					}

				} else {
					if res.currentSeqNo >= seqNo {
						jobDone = true
					}
				}

				if jobDone == true {
					nj--
					ObservePersistPool.Put(res)
				} else {
					// requeue this job
					OPJobChan <- res
				}

			} else {
				// Not currently handling failover scenarios TODO
				nj--
				ObservePersistPool.Put(res)
				failover = true
			}

			if nj == 0 {
				// done with all the jobs
				ok = false
				close(resultChan)
				close(errChan)
			}

		case Err := <-errChan:
			logging.Errorf("Error in Observe/Persist %v", Err.err)
			err = fmt.Errorf("Error in Observe/Persist job %v", Err.err)
			nj--
			ObservePersistPool.Put(Err.job)
			if nj == 0 {
				close(resultChan)
				close(errChan)
				ok = false
			}
		}
	}

	return
}

func (b *Bucket) OPJobPoll() {

	ok := true
	for ok == true {
		select {
		case job := <-OPJobChan:
			pool := b.getConnPoolByHost(job.hostname, false /* bucket not already locked */)
			if pool == nil {
				errRes := &OPErrResponse{vb: job.vb, vbuuid: job.vbuuid}
				errRes.err = fmt.Errorf("Pool not found for host %v", job.hostname)
				errRes.job = job
				job.errorChan <- errRes
				continue
			}
			conn, err := pool.Get()
			if err != nil {
				errRes := &OPErrResponse{vb: job.vb, vbuuid: job.vbuuid}
				errRes.err = fmt.Errorf("Unable to get connection from pool %v", err)
				errRes.job = job
				job.errorChan <- errRes
				continue
			}

			res, err := conn.ObserveSeq(job.vb, job.vbuuid)
			if err != nil {
				errRes := &OPErrResponse{vb: job.vb, vbuuid: job.vbuuid}
				errRes.err = fmt.Errorf("Command failed %v", err)
				errRes.job = job
				job.errorChan <- errRes
				continue

			}
			pool.Return(conn)
			job.lastPersistedSeqNo = res.LastPersistedSeqNo
			job.currentSeqNo = res.CurrentSeqNo
			job.failover = res.Failover

			job.resultChan <- job
		case <-OPJobDone:
			logging.Infof("Observe Persist Poller exitting")
			ok = false
		}
	}
	wg.Done()
}

func (b *Bucket) GetNodeList(vb uint16) []string {

	vbm := b.VBServerMap()
	if len(vbm.VBucketMap) < int(vb) {
		logging.Infof("vbmap smaller than vblist")
		return nil
	}

	nodes := make([]string, len(vbm.VBucketMap[vb]))
	for i := 0; i < len(vbm.VBucketMap[vb]); i++ {
		n := vbm.VBucketMap[vb][i]
		if n < 0 {
			continue
		}

		node := b.getMasterNode(n)
		if len(node) > 1 {
			nodes[i] = node
		}
		continue

	}
	return nodes
}

//pool of ObservePersist Jobs
type OPpool struct {
	pool chan *ObservePersistJob
}

// NewPool creates a new pool of jobs
func NewPool(max int) *OPpool {
	return &OPpool{
		pool: make(chan *ObservePersistJob, max),
	}
}

// Borrow a Client from the pool.
func (p *OPpool) Get() *ObservePersistJob {
	var o *ObservePersistJob
	select {
	case o = <-p.pool:
	default:
		o = &ObservePersistJob{}
	}
	return o
}

// Return returns a Client to the pool.
func (p *OPpool) Put(o *ObservePersistJob) {
	select {
	case p.pool <- o:
	default:
		// let it go, let it go...
	}
}
