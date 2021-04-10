// Copyright 2021 Gian Lorenzo Meocci (glmeocci@gmail.com). All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.
// Run: ./floki-proxy -failure-rate=10 -fail-with-prefix="/small3/aaa"

package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	mathrand "math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/meox/floki-proxy/types"
	log "github.com/sirupsen/logrus"
)

var (
	port                int
	failureRate         int
	failureTransferRate int
	failWithPrefix      types.FailingPrefixCode
	methodCounters      *types.MethodCounters
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	if shouldFail(failureRate) {
		w.WriteHeader(http.StatusInternalServerError)
		log.Warnf("failing request to: %s", r.RequestURI)
		return
	}

	statusCode, failed := shouldFailByPrefix(r.URL.Path)
	if failed {
		w.WriteHeader(statusCode)
		log.Warnf("failing request due to prefix match: %s", r.RequestURI)
		return
	}

	ctx := r.Context()

	// update counters
	methodCounters.Add(r.Method, 1)

	req, err := http.NewRequestWithContext(ctx, r.Method, r.RequestURI, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("creating request: %v", err)
		return
	}

	// attach the original headers
	req.Header = r.Header.Clone()
	req.ContentLength = r.ContentLength
	req.Header.Set("Via", "floki proxy")
	req.Header.Set("X-Forwarded-For", r.RemoteAddr)
	req.Header.Set("X-Forwarded-Host", r.Host)

	// perform the actual request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("performing the request: %v", err)
		return
	}
	defer resp.Body.Close()

	// send back the response header
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	var errorTransfer bool
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		_, errW := w.Write(buf[0:n])
		if errW != nil {
			errorTransfer = true
			break
		}
		if err != nil {
			errorTransfer = true
			break
		}
		if shouldFail(failureTransferRate) {
			// simulate error
			errorTransfer = true
			break
		}
	}

	logger := log.WithField("code", resp.Status).
		WithField("method", r.Method).
		WithField("req-bytes", req.ContentLength).
		WithField("resp-bytes", resp.ContentLength).
		WithField("error-transfer", errorTransfer)

	if resp.StatusCode == http.StatusOK && !errorTransfer {
		logger.Infof("request to %s completed", r.RequestURI)
	} else {
		logger.Warnf("request to %s completed", r.RequestURI)
	}
}

func main() {
	seedRandom()

	flag.IntVar(&port, "port", 9005, "proxy port")
	flag.IntVar(&failureRate, "failure-rate", 0, "percentage of failure")
	flag.IntVar(&failureTransferRate, "failure-transfer-rate", 0, "percentage of failure")
	flag.Var(&failWithPrefix, "fail-with-prefix", "fail all request with the given prefix")
	flag.Parse()

	if failureRate < 0 || failureRate > 100 {
		log.Fatal("bad failure rate: expected a value in the range [0, 100]")
	}

	log.Infof("============== STARTING FLOKI PROXY ==================")
	log.Infof("== Listening on: *:%d", port)
	log.Infof("== F-Rate:    %d%%", failureRate)
	log.Infof("== F-Tr-Rate: %d%%", failureTransferRate)
	log.Infof("== F-Prefix:  %s", failWithPrefix)
	log.Infof("======================================================")

	methodCounters = types.NewMethodCounters()
	go printCounters(context.Background())

	http.HandleFunc("/", mainHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func printCounters(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		methodCounters.PrintCounters()
	}
}

//shouldFail is an utility function the takes as input the failure-rate
//and using a normal distribution decide if the request should
//fails, returning immediately 500, or should be forwarded
func shouldFail(fRate int) bool {
	if fRate == 0 {
		return false
	}
	if fRate == 100 {
		return true
	}

	return mathrand.Intn(100) < failureRate
}

//shouldFailByPrefix if failure by prefix is set return true if the request path
//match the desired prefix, otherwise return false
func shouldFailByPrefix(path string) (int, bool) {
	for k, v := range failWithPrefix {
		if strings.HasPrefix(path, k) {
			return v, true
		}
	}

	return 0, false
}

// seed the random engine using the "/dev/random" as a source
func seedRandom() {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		log.Fatal(err)
	}

	data := binary.BigEndian.Uint64(r[:])
	mathrand.Seed(int64(data))
}
