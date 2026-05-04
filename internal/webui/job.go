package webui

import (
	"sync"
	"sync/atomic"
)

type jobs struct {
	nextID  atomic.Uint32
	idToJob map[uint32]*job
	mutex   sync.RWMutex
}

func newJobs() *jobs {
	return &jobs{
		idToJob: make(map[uint32]*job),
	}
}

func (j *jobs) create() *job {
	numericID := j.nextID.Add(1)
	created := &job{id: numericID}
	j.mutex.Lock()
	j.idToJob[created.id] = created
	j.mutex.Unlock()
	return created
}

func (j *jobs) get(id uint32) (job *job, found bool) {
	j.mutex.RLock()
	defer j.mutex.RUnlock()
	job, found = j.idToJob[id]
	return job, found
}

type job struct {
	id    uint32
	mutex sync.RWMutex
	lines []string
	done  bool
	err   error
}

func (j *job) addLine(line string) {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	j.lines = append(j.lines, line)
}

func (j *job) markDone(err error) {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	j.done = true
	j.err = err
}

func (j *job) snapshot() (lines []string, done bool, err error) {
	j.mutex.RLock()
	defer j.mutex.RUnlock()
	lines = append(lines, j.lines...)
	done = j.done
	err = j.err
	return lines, done, err
}
