// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package exc

import "sync"

// Reporter is used to accumulate and report errors during compilation.
// This is modeled after the protocompile interface of the same name that is
// used to accumulate protobuf compilation errors. The general idea is that
// compilation processes can decide to report an error but continue processing
// rather than fail outright in some cases. The final error set can then be
// shown to the user.
type Reporter interface {
	// Report adds the given record to the set. If this method returns an error
	// then the given error is considered fatal.
	Report(Exception) Exception
	// Reported returns the set of accumulated exceptions.
	Reported() []Exception
}

// NewReporter returns a concurrent-safe implementation of Reporter.
func NewReporter(nonFatal []string) Reporter {
	nf := make(map[string]bool, len(defaultNonFatal))
	for k := range defaultNonFatal {
		nf[k] = true
	}
	for _, k := range nonFatal {
		nf[k] = true
	}
	return &reporterLock{
		Reporter: &reporter{
			nonFatal: nf,
		},
		lock: &sync.Mutex{},
	}
}

type reporter struct {
	reported []Exception
	nonFatal map[string]bool
}

func (r *reporter) Report(e Exception) Exception {
	r.reported = append(r.reported, e)
	if r.nonFatal[e.Code()] {
		return nil
	}
	return e
}

func (r *reporter) Reported() []Exception {
	return r.reported
}

type reporterLock struct {
	Reporter
	lock sync.Locker
}

func (r *reporterLock) Report(e Exception) Exception {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.Reporter.Report(e)
}

func (r *reporterLock) Reported() []Exception {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.Reporter.Reported()
}
