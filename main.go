// Copyright 2021 Gian Lorenzo Meocci (glmeocci@gmail.com). All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.
// Run: ./floki-proxy -failure-rate=10 -fail-with-prefix="/small3/aaa"

package main

import (
	"crypto/rand"
	"encoding/binary"
	"flag"
	mathrand "math/rand"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	failureRate    int
	failWithPrefix string
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("request arrived: %v", r.RemoteAddr)

	if shouldFail() {
		w.WriteHeader(http.StatusInternalServerError)
		log.Warnf("failing request to: %s", r.RequestURI)
		return
	}

	if shouldFailByPrefix(r.URL.Path) {
		w.WriteHeader(http.StatusInternalServerError)
		log.Warnf("failing request due to prefix match: %s", r.RequestURI)
		return
	}

	ctx := r.Context()
	req, err := http.NewRequestWithContext(ctx, r.Method, r.RequestURI, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("creating request: %v", err)
		return
	}

	// attach the original headers
	for k, v := range r.Header {
		for _, x := range v {
			req.Header.Add(k, x)
		}
	}
	req.Header.Add("X-Forwarded-For", r.RemoteAddr)
	req.Header.Add("X-Forwarded-Host", r.Host)
	req.Header.Add("X-Forwarded-Proto", r.Proto)

	// perform the actual request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("performing the request: %v", err)
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		_, errW := w.Write(buf[0:n])
		if errW != nil {
			break
		}
		if err != nil {
			break
		}
	}

	log.WithField("code", resp.Status).
		WithField("bytes", resp.ContentLength).
		Infof("request to %s completed", r.RequestURI)
}

func main() {
	seedRandom()

	flag.IntVar(&failureRate, "failure-rate", 0, "percentage of failure")
	flag.StringVar(&failWithPrefix, "fail-with-prefix", "", "fail all request with the given prefix")
	flag.Parse()

	log.Infof("============== STARTING FLOKI PROXY ==================")
	log.Infof("== F-Rate:   %02d", failureRate)
	log.Infof("== F-Prefix: %s", failWithPrefix)
	log.Infof("======================================================")

	http.HandleFunc("/", mainHandler)
	log.Fatal(http.ListenAndServe(":9005", nil))
}

//shouldFail is an utility function the takes as input the failure-rate
//and using a normal distribution decide if the request should
//fails, returning immediately 500, or should be forwarded
func shouldFail() bool {
	if failureRate == 0 {
		return false
	}
	if failureRate == 100 {
		return true
	}

	bound := float64(failureRate) / 100.0
	return mathrand.NormFloat64() < bound
}

//shouldFailByPrefix if failure by prefix is set return true if the request path
//match the desired prefix, otherwise return false
func shouldFailByPrefix(path string) bool {
	if failWithPrefix == "" {
		return false
	}

	return strings.HasPrefix(path, failWithPrefix)
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
